package k8s

import (
	"context"
	"errors"
	"fmt"
	golive "github.com/apwide/k8s-monitor/pkg/golive"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
	"io"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"net/http"
	"time"
)

func Start(ctx context.Context, kubeconfig *rest.Config, cfg Config) {
	logger := klog.FromContext(ctx)
	logger.Info("Listeners loaded", "count", len(cfg.Listeners))

	// TODO simplify with interface or type assertions to avoid copy type ?
	var goliveClient GoliveDataSender
	goliveClient = &GoliveLoggerSender{logger, cfg.Golive.Yaml}
	if !cfg.Golive.Offline {
		var err error
		goliveClient, err = golive.Golive(ctx, golive.GoliveConfig{
			Url:      cfg.Golive.Url,
			Token:    cfg.Golive.Token,
			Username: cfg.Golive.Username,
			Password: cfg.Golive.Password,
		})
		if err != nil {
			logger.Error(err, "Unable to contact Golive")
			panic(err)
		}
	}

	//logger.Info("Initialized data into Golive")
	//for _, attribute := range cfg.Initialize.EnvironmentAttributes {
	//	goliveClient.Post
	//}

	clientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		panic(err)
	}

	logger.Info("Start listeners")
	handlerList := make([]*Handler, len(cfg.Listeners))
	for i, listener := range cfg.Listeners {
		handlerList[i] = NewHandler(ctx, clientSet, listener, goliveClient, cfg)
	}
	handlers := &Handlers{handlers: handlerList}

	logger.Info("Start Controller")
	informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(clientSet, time.Second*30 /*, kubeinformers.WithNamespace("golive-local")*/)
	controller := NewController(
		ctx,
		clientSet,
		informerFactory.Apps().V1().ReplicaSets(),
		informerFactory.Apps().V1().StatefulSets(),
		informerFactory.Apps().V1().Deployments(),
		informerFactory.Core().V1().Pods(),
		handlers,
	)
	informerFactory.Start(ctx.Done())

	go func() {
		logger.Info("Starting web server")
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "SUCCESS\n")
		})
		http.HandleFunc("/listeners", func(w http.ResponseWriter, r *http.Request) {
			yaml.NewEncoder(w).Encode(cfg.Listeners)
		})
		err := http.ListenAndServe(":9090", nil)
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("Web server closed")
		} else if err != nil {
			logger.Error(err, "Error starting server")
			panic(err)
		}
	}()

	logger.Info("Run Controller")
	if err = controller.run(ctx, 2); err != nil {
		panic(err)
	}
}

type Controller struct {
	replicaSetLister  appslisters.ReplicaSetLister
	statefulSetLister appslisters.StatefulSetLister
	deploymentsLister appslisters.DeploymentLister
	deploymentSync    cache.InformerSynced
	podsLister        corelisters.PodLister
	podSynced         cache.InformerSynced
	// make sure we process only once each element in case multithread + backpressure
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder
	handlers *Handlers
}

func NewController(
	ctx context.Context,
	clientSet *kubernetes.Clientset,
	replicaSetInformer appsinformers.ReplicaSetInformer,
	statefulSetInformer appsinformers.StatefulSetInformer,
	deploymentInformer appsinformers.DeploymentInformer,
	podInformer coreinformers.PodInformer,
	handlers *Handlers) *Controller {

	logger := klog.FromContext(ctx)
	logger.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)
	workqueue := workqueue.NewRateLimitingQueue(ratelimiter)

	controller := &Controller{
		deploymentsLister: deploymentInformer.Lister(),
		replicaSetLister:  replicaSetInformer.Lister(),
		statefulSetLister: statefulSetInformer.Lister(),
		podsLister:        podInformer.Lister(),
		podSynced:         podInformer.Informer().HasSynced,
		workqueue:         workqueue,
		recorder:          recorder,
		handlers:          handlers,
	}

	logger.Info("Setting up event handlers")

	// Set up an event handler for when Foo resources change
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueuePod(new)
		},
		//DeleteFunc:
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) run(ctx context.Context, workers int) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	logger.Info("Starting Golive controller")
	logger.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(ctx.Done(), c.podSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextPod(ctx) {
	}
}

func (c *Controller) enqueuePod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	if c.handlers.isListening(pod) {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
		} else {
			c.workqueue.Add(key)
		}
	}
}

func (c *Controller) processNextPod(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// queue contains namespace/name key (delayed nature of queue make pod unsync with when added to queue)
		if key, ok = obj.(string); !ok {
			// invalid item in queue, just forget it to avoid reprocess loop
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the sync, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.sync(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Info("Successfully synced", "resourceName", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", key)
	}
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "name", name, "namespace", namespace)
	pod, err := c.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.V(2).Info("No longer exists")
			return nil
		}
		return err
	}

	handler := c.handlers.getHandlerFor(pod)
	if handler == nil {
		logger.V(2).Info("No handler")
		return nil
	}
	owner, err := c.findOwner(ctx, pod)
	if err != nil {
		logger.V(2).Error(err, "Ignore pod")
		return nil
	}
	if owner == nil {
		logger.V(2).Info("No related owner found")
		return nil
	}
	metaResource, err := NewMetaResource(owner)
	if err != nil {
		return err
	}
	handler.Handle(metaResource)
	// c.recorder.Event(pod, corev1.EventTypeNormal, SuccessSynced, MessageSynced)
	return nil
}

func (c *Controller) findOwner(ctx context.Context, pod *corev1.Pod) (interface{}, error) {
	name := pod.GetName()
	namespace := pod.GetNamespace()
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "name", name, "namespace", namespace)
	pod, err := c.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		// TODO no controller => deal with pod
		logger.V(2).Info("Pod without owner is ignored")
		return nil, nil
	}
	switch owner.Kind {
	case "Job":
		{
			logger.V(2).Info("Pod owned by Job is ignored")
			return nil, nil
		}
	case "ReplicaSet":
		{
			replicaSet, err := c.replicaSetLister.ReplicaSets(namespace).Get(owner.Name)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					logger.V(2).Info("Pod owned by a not found ReplicaSet")
					return nil, nil
				}
				return nil, err
			}

			owner = metav1.GetControllerOf(replicaSet)
			if owner == nil {
				logger.V(2).Info("Pod owned by ReplicaSet without owner is ignored")
				return nil, nil
			}

			deployment, err := c.deploymentsLister.Deployments(namespace).Get(owner.Name)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					logger.V(2).Info("Pod owned by a not found Deployment")
					return nil, nil
				}
				return nil, err
			}

			return deployment, nil
		}
	case "StatefulSet":
		{
			statefulSet, err := c.statefulSetLister.StatefulSets(namespace).Get(owner.Name)
			if err == nil {
				return statefulSet, nil
			} else if k8serrors.IsNotFound(err) {
				logger.V(2).Info("Pod owned by a not found StatefulSet")
				return nil, nil
			}
			return nil, err
		}
	default:
		return nil, fmt.Errorf("uknown owner Kind %s for pod %s/%s", owner.Kind, namespace, name)
	}
}

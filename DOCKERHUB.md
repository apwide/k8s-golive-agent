# What is Apwide Golive for Jira?

Apwide Golive is the game-changing solution for comprehensive test environment management.

Seamlessly integrated with your development and release processes, Apwide Golive empowers your teams to
deliver high-quality software with speed and confidence.
Apwide Golive provides a centralized dashboard for visualizing and tracking environment usage, resource allocation, and availability right in Jira.

With integrated notifications and approvals in Jira, stakeholders stay informed and can provide quick feedback, reducing delays and accelerating the testing
process.

Learn more about Apwide Golive: https://www.apwide.com

# Configuration

Configure is done using a yaml file.
All of the values can be overriden by environment variables such as:
```yaml
golive:
  offline: true
```
can be overridden with:
```shell
GOLIVE_OFFLINE=true
```

## Golive

By default, operator runs in offline mode, which logs the data supposed to be sent to Golive. This will help you to first configure what to track and
how to extract and send data to Golive.


Most simple configuration is:
```yaml
golive:
  offline: true
listeners:
  - id: my-listener
```
This configuration will track each pod of each namespace and log json payload that would be sent to Golive if not in offline.
At least one listener is mandatory.

Offline mode supports logging Golive payload in yaml which is more readable in console logs:
```yaml
golive:
  offline: true
  yaml: true
```

When you are ready, you can disable the offline mode and configure Golive API client to push information:

### Cloud
For cloud, you only need an [authentication token](https://www.apwide.com/golive/cloud/environments/help/how-to-use-api-tokens)
```yaml
golive:
  offline: false
  token: "eyJra[...]xKWvNoQ"
```

### Server
For server, you need to specify the Golive base API url for your given Jira instance and its credentials:
```yaml
golive:
  offline: false
  # https://[jiraBaseUrl]:[jiraPort]/[jiraContextPath]/rest/apwide/tem/1.1
  url: https://jira.company.com/rest/apwide/tem/1.1
  username: golive-service-account
  password: XXXX
```

## Listeners

Listeners specify what to track, using *selectors*, what/how to extract environment/deployment data and push them to Golive.

A listener is composed of:
* **id**: a unique identifier which helps in the logs to know which config has been used
* **autoCreate**: a boolean to specific if target(s) (application, category and environment) must be created if not existing into Golive if call should fail.
* **category**: how to extract/select category.
* **application**: how to extract/select application.
* **name**: how to extract/select environment name.
* **attributes**: what and how to extract environment attribute values
* **selectors**: pod selectors used to select which event should match this configuration.

## Selectors

Selectors are applied on pod to check if they are monitored or not:
```yaml
listeners:
  - id: frontend-monitoring
    selectors:
      - namespace: app-dev
        labels:
          app: dev
        labelQuery: app in (dev)
```

3 type of selectors are currently supported:
* **namespace**: restrict to pod in the given namespace.
* **labels**: a map key/value that must matches a pod (logical and).
* **labelQuery**: a [set-based expression](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#set-based-requirement) that pod labels must match.

Pod must satisfy each of the criteria (namespace, label, labelQuery) specified in a given selector,
but a listener can have several **selectors**, and pod must match all of the criteria of only one of them.
In case a pod match multiple selectors in different listeners, the first declared listener wins.

## Data Extractions

The following sections defines how to extract environment information.
k8s-golive-monitor is litening pod events to keep track of status/info, but to extract data,
it is interested in the pod owner resource: Deployment, StatefulSet, DaemonSet.
Only these 3 are currently supported (Job, single Pod... are ignored).

When a pod match a selector, k8s-golive-monitor find owning resource, and apply the extraction
rules on it to get application, category, version...

This means when a configuration requires to extract information from a label, annotation, jsonpath,
the handler will work at owner level (eg: Deployment...).

An exception, is, if the configuration specifies we want to apply the rules at namespace.
It is common the use namespace to design/categorizes the topology of our environments (eg: Dev, QA, Staging).
That's, k8s-golive-monitor can extract information from namespace, if configured for.

## Category

Category section is to extract Golive category name:
```yaml
listeners:
  - id: my-listener
    category:
      namespace: true
      value: "Dev"
      label: "my.company.com/cat"
      annotation: "my.company.com/env"
      template: |
        {{ .Value }} ({{ label "my.company.com/cat"  }})
```

### Parameters
* **namespace** : boolean to specify if we want to search for label/annotation/template at namespace level or owner (eg: Deployment)
* **value** : hard-coded value taken as is from the configuration
* **label** : name taken from value of this label
* **annotation** : value of this annotation used
* **template** : possibility to customize the value with [template expression](#template-expression)

For the template expression, here is the available context element:
* **Value** : value matched by the previous rule


### Precedence
In case a parameter is specified and doesn't match, an error is raised and data are not sent.

Precedence order to get category name when namespace=true
1. **value** from config
2. **label** namespace
3. **annotation** from namespace
4. **golive.apwide.net/cat** default label on namespace
5. **golive.apwide.net/cat** default annotation on namespace
6. **namespace name** in last resort

Precedence order to get category name when namespace=false
1. **value** from config
2. **label** on owning resource
3. **annotation** on owning resource
4. **golive.apwide.net/cat** default label on owning resource
5. **golive.apwide.net/cat** default annotation on owning resource

## Application

Application section is to extract Golive application name:
```yaml
listeners:
  - id: my-listener
    application:
      namespace: true
      value: "Payment"
      label: "my.company.com/app"
      annotation: "my.company.com/env-name"
      template: |
        {{ .Value }} ({{ label "my.company.com/app"  }})
```

### Parameters
* **namespace** : boolean to specify if we want to search for label/annotation/template at namespace level or owner (eg: Deployment)
* **value** : hard-coded value taken as is from the configuration
* **label** : name taken from value of this label
* **annotation** : value of this annotation used
* **template** : possibility to customize the value with [template expression](#template-expression)

For the template expression, here is the available context element:
* **Value** : value matched by the previous rule

### Precedence
In case a parameter is specified and doesn't match, an error is raised and data are not sent.

Precedence order to get application name when namespace=true
1. **value** from config
2. **label** namespace
3. **annotation** from namespace
4. **golive.apwide.net/app** default label on namespace
5. **golive.apwide.net/app** default annotation on namespace
6. **namespace name** in last resort

Precedence order to get application name when namespace=false
1. **value** from config
2. **label** on owning resource
3. **annotation** on owning resource
4. **golive.apwide.net/app** default label on owning resource
5. **golive.apwide.net/app** default annotation on owning resource
6. **docker image name** in last resort

## Version

Version section is to extract Golive deployment version name:
```yaml
listeners:
  - id: my-listener
    version:
      ignore: false
      namespace: true
      value: "1.0"
      label: "my.company.com/app"
      annotation: "my.company.com/env-name"
      template: |
        {{ .Value }}-SNAPSHOT
```

### Parameters
* **ignore** : in case deployment should not be tracked into Golive.
* **namespace** : boolean to specify if we want to search for label/annotation/template at namespace level or owner (eg: Deployment)
* **value** : hard-coded value taken as is from the configuration
* **label** : name taken from value of this label
* **annotation** : value of this annotation used
* **template** : possibility to customize the value with [template expression](#template-expression)

For the template expression, here is the available context element:
* **Value** : value matched by the previous rule

### Precedence
In case a parameter is specified and doesn't match, an error is raised and data are not sent.

Precedence order to get version name when namespace=true
1. **value** from config
2. **label** namespace
3. **annotation** from namespace
4. **golive.apwide.net/app** default label on namespace
5. **golive.apwide.net/app** default annotation on namespace
6. **namespace name** in last resort

Precedence order to get application name when namespace=false
1. **value** from config
2. **label** on owning resource
3. **annotation** on owning resource
4. **golive.apwide.net/app** default label on owning resource
5. **golive.apwide.net/app** default annotation on owning resource
6. **docker image version** in last resort

## Name
Name section is to extract Golive environment name:
```yaml
listeners:
  - id: my-listener
    name:
      namespace: true
      value: "1.0"
      label: "my.company.com/app"
      annotation: "my.company.com/env-name"
      template: |
        {{ .Value }}-SNAPSHOT
```

### Parameters
* **namespace** : boolean to specify if we want to search for label/annotation/template at namespace level or owner (eg: Deployment)
* **value** : hard-coded value taken as is from the configuration
* **label** : name taken from value of this label
* **annotation** : value of this annotation used
* **template** : possibility to customize the value with [template expression](#template-expression)

For the template expression, here is the available context element:
* **Value** : value matched by the previous rule
* **App.Name** : application name as processed by previous section
* **Cat.Name** : category name as processed by previous section

### Precedence
In case a parameter is specified and doesn't match, an error is raised and data are not sent.

Precedence order to get environment name when namespace=true
1. **value** from config
2. **label** namespace
3. **annotation** from namespace
4. **golive.apwide.net/app** default label on namespace
5. **golive.apwide.net/app** default annotation on namespace
6. **namespace name**

Precedence order to get environment name when namespace=false
1. **value** from config
2. **label** on owning resource
3. **annotation** on owning resource
4. **golive.apwide.net/app** default label on owning resource
5. **golive.apwide.net/app** default annotation on owning resource
6. **owning resource name** in last resort

## Environment Attributes
An array of expression to extract environment attributes:
```yaml
listeners:
  - id: my-listener
    environmentAttributes:
      - name: Team
        value: Apwide
      - name: Environment Variable
        fromPath: .spec.template.spec.containers[0].env[?(@.name == 'MY_ENV_VARIABLE')].value
```

### Parameters

* **name**: name of the attribute. (attribute must exists into Golive)
* **value**: configuration hard-coded value for the given attribute.
* **fromPath**: json path expression evaluated on owning resource

## Status
To keep track of your environment statuses, you have to map operator status to Golive status.

Operator evalute the state of an environment using 4 different statuses:
* down: environment is not running (eg: deployment replicas set to 0)
* up: up and running (eg: deployment desired and read replicas are equals)
* deploy: a change is ongoing (eg: deployment is starting up or a new deployment update happens and pods are restarting)
* failed: deploymennt failed (eg: deployment has been updated, but pod are not able to start)

Golive status can be identified by their id and/or name.
```yaml
statusMapping:
  down:
    name: Down
  up:
    name: Up
  deploy:
    name: Deploy
  failed:
    name: Down
```

## Template Expression

In various part of configuration, [go template expression](https://pkg.go.dev/text/template) can be used
to transform value before sending them to golive.

In addition to the standard go template function, here are the additional one provided:
* **title** : https://pkg.go.dev/strings#ToTitle
* **lower** : https://pkg.go.dev/strings#ToLower
* **upper** : https://pkg.go.dev/strings#ToUpper
* **annotation** : read annotation value from the owned resource
* **label** : read label value from the owned resource
* **jsonPath** : evaluate the jsonPath expression on the owned resource

# Limitations

## Monitored Resources

The operator listens pods for events, but it capture information/status from its owner perspective.

This means pod created manually which do not have owner are ignored.

Current ownership resource type supported are:
* DaemonSet
* StatefueSet
* Deployment

## Deployment Date

Currently, *Deployment Date* is set to *now*, so, in case you have enabled the option "Track re-deployments of the same version", each time Golive is updated
by the operator, a new deployment will be generated.

## Attributes

If you are populating attributes, for the moment, you have to make sure they exist in Golive and create them before.

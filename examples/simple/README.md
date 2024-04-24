# Simple

[Given configuration](./config.yaml) has :
* listener any pod events in namepsace **app-dev**
* **autoCreate** target elements such as application, category and environment if they do not exist into Golive.
* extract **category** information from the **namespace**

Based on the example [app](../app/app.yaml), here is what will be pushed to Golive:
```yaml
deployment:
    attributes: {}
    buildnumber: null
    deployeddate: "2024-04-24T13:56:07Z"
    description: null
    issues: null
    versionid: null
    versionname: 1.14.2
environment:
    attributes:
        CustomAttribute: Deployment Attribute Value
        cat: App Dev
    name: payment-app
    url: null
environmentselector:
    application:
        autocreate: true
        id: null
        name: nginx
    category:
        autocreate: true
        id: null
        name: Default Namespace Label
    environment:
        autocreate: true
        id: null
        name: payment-app
status: null
```

We can see that:
* **deployment.versionname** has been taken from docker image version
* **environment.attributes** has been populated with the 


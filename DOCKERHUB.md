# What is Apwide Golive for Jira?

Apwide Golive is the game-changing solution for comprehensive test environment management.

Seamlessly integrated with your development and release processes, Apwide Golive empowers your teams to
deliver high-quality software with speed and confidence.
Apwide Golive provides a centralized dashboard for visualizing and tracking environment usage, resource allocation, and availability right in Jira.

With integrated notifications and approvals in Jira, stakeholders stay informed and can provide quick feedback, reducing delays and accelerating the testing
process.

Learn more about Apwide Golive: https://www.apwide.com

## Configuration

### Golive

By default, operator runs in offline mode, which logs the data supposed to be sent to Golive. This will help you to first configure what to track and
how to extract and send data to Golive. Offline mode supports logging Golive payload in yaml or json:
```yaml
golive:
  offline: true
  yaml: true
```

When you are ready, you can disable the offline mode and configure Golive API client to push information:

#### Cloud
For cloud, you only need an [authentication token](https://www.apwide.com/golive/cloud/environments/help/how-to-use-api-tokens)
```yaml
golive:
  offline: false
  token: "eyJra[...]xKWvNoQ"
```

#### Server
For server, you need to specify the Golive base API url for your given Jira instance and its credentials:
```yaml
golive:
  offline: false
  # https://[jiraBaseUrl]:[jiraPort]/[jiraContextPath]/rest/apwide/tem/1.1
  url: https://jira.company.com/rest/apwide/tem/1.1
  username: golive-service-account
  password: XXXX
```

### Listeners

Listeners specify what to track, using *selectors*, and what to extract and push to Golive.

A listener is composed of:
* **id**: a unique identifier which helps in the logs to know which config has been used
* **autoCreate**: a boolean to specific if target(s) (application, category and environment) must be created if not existing into Golive if call should fail.
* **category**: how to extract/select category.
* **application**: how to extract/select application.
* **name**: how to extract/select environment name.
* **attributes**: what and how to extract environment attribute values
* **selectors**: pod selectors used to select which event should match this configuration.

#### Simple example

Imagine we have this topology:
* We use k8s namespace to separate our environments (eg: Dev, QA, Staging, Prod...)
* We have a web application deployed using a deployment resource.

We have this selector

### Status
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

## Limitations

### Monitored Resources

The operator listens pods for events, but it capture information/status from its owner perspective.

This means pod created manually which do not have owner are ignored.

Current ownership resource type supported are:
* DaemonSet
* StatefueSet
* Deployment

### Deployment Date

Currently, *Deployment Date* is set to *now*, so, in case you have enabled the option "Track re-deployments of the same version", each time Golive is updated
by the operator, a new deployment will be generated.

### Attributes

If you are populating attributes, for the moment, you have to make sure they exist in Golive and create them before.

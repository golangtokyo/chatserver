A simple chat server on Google App Engine. The messages are stored at memcache and might be disappeared.

## API

### POST /

```json
{"name":"your name","body":"message body"}
```

## How to test this app on your local machine

### Install Cloud SDK

See https://cloud.google.com/appengine/docs/standard/go/quickstart

### Run this app

```shell
dev_appserver.py app.yaml
```

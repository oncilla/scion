# SVC request tool

The `svcreq` application is a simple tool to send requests to a 
service. It can be used to check that services get the requests.
For example, it can be used to check that the service is still
reachable after the border router reloads the topology file.

There are two modes:
```
svcreq -local 1-ff00:0:111,[127.0.0.1]:4002 -remote 1-ff00:0:110,[CS] -log.console info
```

Sends a request to the specified svc address in the remote AS. The request
type is inferred from the svc address.

```
svcreq -local 1-ff00:0:111,[127.0.0.1]:4002 -remote 1-ff00:0:110,[127.0.0.2]:30084 \
-svc CS -log.console info
```

Sends a request to the specified remote address. The request 
type is inferred from the svc flag.
### Login container By docker/kubernetes

####Configure

```bash
if use kubeclient : tty/app.go: 102~105, config kube Info
if use dockerclient: tty/docker_exec.go: 24,34 config docker api address and container uid/name
```

#### Run

```bash
go build
./container-tty
# open browser, and enter 127.0.0.1:8080
```

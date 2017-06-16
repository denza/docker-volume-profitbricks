[![Go Report Card](https://goreportcard.com/badge/github.com/vieux/docker-volume-sshfs)](https://goreportcard.com/badge/github.com/stackpointcloud/docker-volume-profitbricks)

# docker-volume-profitbricks
Docker volume plugin for ProfitBricks


Deploy:

```bash
export PROFITBRICKS_USERNAME="username"
export PROFITBRICKS_PASSWORD="password"
export PROFITBRICKS_DATACENTER="datacenter-uuid"

```
```
$ go get github.com/StackPointCloud/docker-volume-profitbricks
$ cd $GOPATH/src/github.com/StackPointCloud/docker-volume-profitbricks
$ go build
$ ./docker-volume-profitbricks

```

Create a docker volume with profitbricks:
```bash
docker volume create --driver profitbricks --name test02
#Mount the volume and start interactive shell to access contents of your ProfitBricks volume from within a container
docker run -ti --rm --volume test02:/mydata busybox sh
```

Once inside the container:
```bash
echo "hello world" > /mydata/hello.txt
cat /mydata/hello.txt
 hello world
```

The current status of the Docker volume can be inspected using the following command:
```bash
docker volume inspect test02
```
```json
[
    {
        "Driver": "profitbricks",
        "Labels": {},
        "Mountpoint": "/var/lib/docker-volume-profitbricks/dev/vdb",
        "Name": "/dev/vdb",
        "Options": {},
        "Scope": "local"
    }
]
```

#System Integration

Edit [profitbricks.service](profitbricks.service):
```bash
[Unit]
Description=Docker Volume Driver for ProfitBricks
Before=docker.service
After=network.target
Requires=docker.service

[Service]
ExecStart=/root/go/src/github.com/StackPointCloud/docker-volume-profitbricks/docker-volume-profitbricks -d [datacenter_uuid] -u [profitbricks_username] -p [profitbricks_password]

[Install]
WantedBy=multi-user.target
```

and copy it 
```bash
cp profitbricks.service /etc/systemd/system
```

Start the service:
```bash
# execute the driver directly
sudo systemctl start profitbricks.service

# enable automated startup on reboot
sudo systemctl enable profitbricks.service
```
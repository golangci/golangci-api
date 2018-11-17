sudo apt-get remove runc # caused errors with containerd.io install
# install docker v=18.09.0

# https://github.com/google/gvisor#download-a-build
wget https://storage.googleapis.com/gvisor/releases/nightly/2018-11-16/runsc
wget https://storage.googleapis.com/gvisor/releases/nightly/2018-11-16/runsc.sha512
sha512sum -c runsc.sha512
chmod a+x runsc
sudo mv runsc /usr/local/bin

cat <<EOT >> /etc/docker/daemon.json
{
    "runtimes": {
        "runsc": {
            "path": "/usr/local/bin/runsc"
        }
    }
}
EOT
sudo systemctl restart docker
docker run --rm --runtime=runsc hello-world
echo "stopping docker-control-agent"
docker stop docker-control-agent
echo "removing docker-control-agent"
docker rm docker-control-agent
echo "pulling latest changes"
git pull origin master
echo "building new image"
docker build -t xiovv/docker-control-agent:latest .
echo "running new container"
docker run -p 5006:8080 --name=docker-control-agent -v /var/run/docker.sock:/var/run/docker.sock xiovv/docker-control-agent:latest
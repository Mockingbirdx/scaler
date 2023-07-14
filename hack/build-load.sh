docker buildx build --platform linux/amd64 -t registry.cn-hangzhou.aliyuncs.com/lullaby/scaler:v0.0.1 .
kind load docker-image registry.cn-hangzhou.aliyuncs.com/lullaby/scaler:v0.0.1
docker image prune
./hack/run.sh
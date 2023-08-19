docker tag registry.cn-hangzhou.aliyuncs.com/lullaby/scaler:v0.0.1 registry.cn-hangzhou.aliyuncs.com/lullaby/scaler:v$1
docker push registry.cn-hangzhou.aliyuncs.com/lullaby/scaler:v$1
docker image rm registry.cn-hangzhou.aliyuncs.com/lullaby/scaler:v$1
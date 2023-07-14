kubectl delete -f hack/serverless-simulaion.yaml
kubectl apply -f ./hack/serverless-simulaion.yaml

# data_3 68 min
# kubectl exec jobs/serverless-simulation -c scaler -- curl http://127.0.0.1:9000/
./hack/build.sh
./hack/test.sh $1 $2

# sed -e 'N;34a\        args: ["dataSet_'$1'"]' ./hack/job-template.yaml > ./hack/job-test-$2.yaml 
# sed -i 'N;4a\  name: test-'$2'' ./hack/job-test-$2.yaml #> ./hack/job-test-$2.yaml 

# kubectl delete -f ./hack/job-test-$2.yaml
# kubectl apply -f  ./hack/job-test-$2.yaml

# kubectl exec jobs/serverless-simulation -c scaler -- curl http://127.0.0.1:9000/
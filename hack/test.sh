sed -e 'N;34a\        args: ["dataSet_'$1'"]' ./hack/job-template.yaml > ./hack/job-test-$2.yaml 
sed -i 'N;4a\  name: test-'$2'' ./hack/job-test-$2.yaml #> ./hack/job-test-$2.yaml 

kubectl delete -f ./hack/job-test-$2.yaml
kubectl apply -f  ./hack/job-test-$2.yaml
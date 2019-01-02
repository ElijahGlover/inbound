echo "Building with GOPATH $GOPATH"

echo "Building..."
mkdir -p bin
go build -o bin/inbound ./cmd/inbound
OUT=$?

if [ $OUT -eq 0 ];then
   echo "Build success"
else
   echo "Build failed, exit code $OUT"
   exit $OUT
fi

echo "Running..."
export TARGET_NAMESPACE=core
export KUBECONFIG=/Users/elijahglover/.kube/config
export LOG_LEVEL=verbose
./bin/inbound
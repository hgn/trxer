all:
	GOOS=linux GOARCH=amd64 go build trxer.go
	GOOS=windows GOARCH=amd64 go build -o trxer.exe trxer.go

clean:
	rm -rf trxer

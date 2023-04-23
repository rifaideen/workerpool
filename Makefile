benchmark:
	go test -v -bench=.

coverage:
	mkdir -p report
	go test -v -covermode=count -coverpkg=. -coverprofile report/coverage.out .
	go tool cover -html report/coverage.out -o report/coverage.html


.PHONY: coverage
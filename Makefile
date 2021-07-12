test: test-sort test-sort-cleanup test-sort-cleanup-update

test-sort:
	cat example/ingress.yml | go run . update -v="" --cleanup="" | diff example/ingress.sort.yml.golden -

test-sort-cleanup:
	cat example/ingress.yml | go run . update -v="" | diff example/ingress.sort.cleanup.yml.golden -

test-sort-cleanup-update:
	cat example/ingress.yml | go run . update | diff example/ingress.sort.cleanup.update.yml.golden -

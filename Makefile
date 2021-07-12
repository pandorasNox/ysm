test: test-update test-update-rm

test-update:
	cat example/ingress.yml | go run . update -v="" --cleanup="" | diff example/ingress.sort.yml.golden -

test-update-rm:
	cat example/ingress.yml | go run . update -v="" | diff example/ingress.sort.cleanup.yml.golden -

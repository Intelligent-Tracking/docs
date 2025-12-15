generate:
	@npx --yes @mintlify/scraping openapi-file ./api.openapi.json -o ./tmp > /dev/null
	@go run processor.go
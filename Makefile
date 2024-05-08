wga: .PHONY
	go build

# https://github.com/bitnami/readme-generator-for-helm
# We love javascript applications!
readme: .PHONY
	readme-generator --values ./charts/wga/values.yaml --readme ./README.md

gen: .PHONY
	go run codegen/main.go


.PHONY: 

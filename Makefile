wga: .PHONY
	go build

# https://github.com/bitnami/readme-generator-for-helm
# We love javascript applications!
readme: .PHONY
	readme-generator --values ./charts/wga/values.yaml --readme ./README.md

# # go install k8s.io/code-generator/cmd/client-gen@v0.30.1
# client: .PHONY
# 	client-gen \
# 		--input-base="github.com/kraudcloud/wga/pkgs" \
# 		--input="apis/v1beta" \
# 		--output-base="pkgs/clients" \
# 		--output-package="github.com/kraudcloud/wga/pkgs/clients" \
# 		--clientset-name="v1beta" \
# 		--fake-clientset=false \
# 		--go-header-file=boilerplate.go.txt \

# go install k8s.io/code-generator/cmd/deepcopy-gen@v0.30.1
copy: .PHONY
	deepcopy-gen \
		--go-header-file=boilerplate.go.txt \
		--input-dirs ./pkgs/apis/v1beta \

# go install k8s.io/code-generator/cmd/register-gen@v0.30.1
register: .PHONY
	register-gen \
		--go-header-file=boilerplate.go.txt \
		--input-dirs ./pkgs/apis/v1beta \

gen: register copy

.PHONY: 

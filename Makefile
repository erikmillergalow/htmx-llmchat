BINARY_NAME=htmx-llmchat

BUILD_DIR=./build

TARGETS = \
	darwin-amd64 \
	darwin-arm64 \
	windows-amd64 \
	linux-amd64

.PHONY: all clean $(TARGETS)

all: clean $(TARGETS)

clean:
	rm -rf $(BUILD_DIR)

$(TARGETS):
	$(eval OS := $(word 1,$(subst -, ,$@)))
	$(eval ARCH := $(word 2,$(subst -, ,$@)))
	$(eval DIR_NAME := $(if $(filter darwin,$(OS)),mac,$(OS))-$(ARCH))
	templ generate
	mkdir -p $(BUILD_DIR)/$(BINARY_NAME)-$(DIR_NAME)
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(BUILD_DIR)/$(BINARY_NAME)-$(DIR_NAME)/$(BINARY_NAME)$(if $(filter windows,$(OS)),.exe,)
	cd $(BUILD_DIR) && zip -r $(BINARY_NAME)-$(DIR_NAME).zip $(BINARY_NAME)-$(DIR_NAME)
	
windows-amd64:
	mkdir -p $(BUILD_DIR)/$(BINARY_NAME)-$@
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-$@/$(BINARY_NAME).exe
	cd $(BUILD_DIR) && zip -r $(BINARY_NAME)-$@.zip $(BINARY_NAME)-$@

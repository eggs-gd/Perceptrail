
GOMODULES = \
	gontroller \
	perceplib/logger \
	perceplib/perceptors \
	perceptors/color \
	perceptors/geo \
	perceptors/faces \
	perceptors/objects


all: build-gontroller build-plugins


build-gontroller:
	@echo "Building gontroller..."
	@$(MAKE) -C gontroller


build-plugins: $(addprefix build-plugin-,$(notdir $(filter perceptors/%,$(GOMODULES))))

build-plugin-%:
	@echo "Building plugin: $*..."
	@cd perceptors/$* && go build -buildmode=plugin -o ../../gontroller/build/plugins/$*.so


update-deps:
	@for module in $(GOMODULES); do \
		echo "Updating dependencies for $$module..."; \
		(cd $$module && go mod tidy); \
	done


clean:
	@echo "Cleaning all build artifacts..."
	@rm -rf gontroller/build/plugins/*.so
	@for module in $(GOMODULES); do \
		echo "Cleaning $$module..."; \
		(cd $$module && go clean); \
	done

.PHONY: all build-gontroller build-plugins build-plugin-% update-deps clean


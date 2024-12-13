GOMODULES = \
	perceplib \
	gontroller \
	perceptors/exif_geo \
	perceptors/ml_color \
	perceptors/ml_faces \
	perceptors/ml_objects

.PHONY: all build-gontroller build-plugins build-plugin-% update-deps clean

all: clean update-deps build-gontroller build-plugins

build-gontroller:
	@echo "Building gontroller..."
	@$(MAKE) -C gontroller

build-plugins: $(addprefix build-plugin-, $(notdir $(filter perceptors/%,$(GOMODULES))))

build-plugin-%:
	@echo "Building plugin: $*..."
	@mkdir -p ../../gontroller/build/plugins
	@cd perceptors/$* && go build -buildmode=plugin -o ../../gontroller/build/plugins/$*.so

update-deps:
	@$(foreach module, $(GOMODULES), \
		echo "Updating dependencies for $(module)..."; \
		(cd $(module) && go mod tidy); \
	)

clean:
	@echo "Cleaning all build artifacts..."
	@rm -rf gontroller/build/plugins/*.so
	@$(foreach module, $(GOMODULES), \
		echo "Cleaning $(module)..."; \
		(cd $(module) && go clean); \
	)

test:
	@echo "Running tests..."
	@$(foreach module, $(GOMODULES), \
		echo "Testing $(module)..."; \
		(cd $(module) && go test ./...); \
	)
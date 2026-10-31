GOMODULES = \
	perceplib \
	gontroller \
	perceptors/exif_geo \
	perceptors/ml_color \
	perceptors/ml_faces \
	perceptors/ml_objects

.PHONY: all build-gontroller build-plugins build-plugin-% update-deps clean test sonar sonar-publish

all: clean update-deps build-gontroller build-plugins

build-gontroller:
	@echo "Building gontroller..."
	@$(MAKE) -C gontroller

build-plugins: $(addprefix build-plugin-, $(notdir $(filter perceptors/%,$(GOMODULES))))

build-plugin-%:
	@echo "Building plugin: $*..."
	@mkdir -p gontroller/.build/plugins
	@cd perceptors/$* && go build -buildmode=plugin -o ../../gontroller/.build/plugins/$*.so

update-deps:
	@$(foreach module, $(GOMODULES), \
		echo "Updating dependencies for $(module)..."; \
		(cd $(module) && go mod tidy); \
	)

clean:
	@echo "Cleaning all build artifacts..."
	@rm -rf gontroller/.build/plugins/*.so
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

# SonarQube on the owner's LAN server: every Go module's tests with coverage (an
# integration test counts for the packages it runs: -coverpkg) and golangci-lint's
# findings (.golangci.yml: a report, not a stop — the gate decides), svebapp's unit
# tests with coverage (lcov), the scan, the quality gate (red: a non-zero exit). SONAR_HOST_URL and SONAR_TOKEN from the
# environment or a git-ignored .env; never printed.
# -short: the tests that load the perceptors as plugins skip (pluginbuild) — a host
# built for coverage loads no plugin, built with -cover or not; go test runs them
# (pre-push, CI).
SONAR_GOMODULES = $(GOMODULES) scripts/declorder
# SONAR_GATE=false sends the report without waiting for the quality gate.
SONAR_GATE ?= true

sonar:
	@env=./.env; [ -f $$env ] || env="$$(git rev-parse --path-format=absolute --git-common-dir)/../.env"; \
	set -a; [ ! -f "$$env" ] || . "$$env"; set +a; \
	test -n "$$SONAR_HOST_URL" && test -n "$$SONAR_TOKEN" || { echo 'sonar: set SONAR_HOST_URL and SONAR_TOKEN (.env)' >&2; exit 1; }; \
	scope=$$(scripts/sonar-scope) || exit 1; \
	echo "sonar: $${scope:-main branch}"; \
	for module in $(SONAR_GOMODULES); do \
		echo "Coverage: $$module"; \
		(cd $$module && go test -short -coverpkg=./... -coverprofile=coverage.out ./... > /dev/null) || exit 1; \
		(cd $$module && golangci-lint run --issues-exit-code=0 --output.text.path=stderr \
			--output.checkstyle.path=$(CURDIR)/$$module/golangci-lint.xml ./... 2> /dev/null) || exit 1; \
	done; \
	echo "Coverage: svebapp"; \
	(cd svebapp && npx vitest run --coverage > /dev/null) || exit 1; \
	{ if [ -z "$$(git status --porcelain --untracked-files=no)" ]; then git rev-parse HEAD > "$$(git rev-parse --git-path sonar-reports-head)"; else rm -f "$$(git rev-parse --git-path sonar-reports-head)"; fi; }; \
	sonar-scanner -Dsonar.qualitygate.wait=$(SONAR_GATE) $$scope

# The scan again, on the reports `make sonar` wrote for this commit, once the commit is on GitHub:
# the pull request's check is made for a commit GitHub knows, and at pre-push it does not yet.
sonar-publish:
	@env=./.env; [ -f $$env ] || env="$$(git rev-parse --path-format=absolute --git-common-dir)/../.env"; \
	set -a; [ ! -f "$$env" ] || . "$$env"; set +a; \
	test -n "$$SONAR_HOST_URL" && test -n "$$SONAR_TOKEN" || { echo 'sonar-publish: set SONAR_HOST_URL and SONAR_TOKEN (.env)' >&2; exit 1; }; \
	[ -z "$$(git status --porcelain --untracked-files=no)" ] || { echo 'sonar-publish: uncommitted changes: the scan would not match the reports of the commit' >&2; exit 1; }; \
	[ "$$(cat "$$(git rev-parse --git-path sonar-reports-head)" 2>/dev/null)" = "$$(git rev-parse HEAD)" ] || { echo 'sonar-publish: no reports of this commit: run make sonar first' >&2; exit 1; }; \
	git ls-remote origin "refs/heads/$$(git rev-parse --abbrev-ref HEAD)" | grep -q "^$$(git rev-parse HEAD)" || { echo 'sonar-publish: this commit is not on the remote yet: push it first' >&2; exit 1; }; \
	scope=$$(scripts/sonar-scope) || exit 1; \
	echo "sonar: $${scope:-main branch} (the reports of this commit, sent again)"; \
	sonar-scanner -Dsonar.qualitygate.wait=false $$scope

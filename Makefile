build:
    # Build the gontroller binary
    go build -o gontroller/bin/gontroller gontroller/main.go
    # Build plugins and copy to gontroller/bin/plugins
    mkdir -p gontroller/bin/plugins
    for plugin in perceptors/src/*; do \
        go build -buildmode=plugin -o perceptors/bin/$$(basename $$plugin).so $$plugin; \
        cp perceptors/bin/$$(basename $$plugin).so gontroller/bin/plugins/; \
    done
    # Build the web app
    cd svebapp && npm run build

run: build
    ./gontroller/bin/gontroller & cd svebapp && npm run dev

clean:
    rm -rf gontroller/bin gontroller/bin/plugins perceptors/bin/*.so svebapp/public/build

test:
    go test ./gontroller/... ./perceptors/lib/... && cd svebapp && npm test

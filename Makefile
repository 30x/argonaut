#Format is MAJOR . MINOR . PATCH

VERSION=1.0.0

release: dir-build package-linux package-windows package-darwin

package-linux: dir-linux build-linux zip-linux

package-windows: dir-windows build-windows zip-windows

package-darwin: dir-darwin build-darwin zip-darwin

dir-build:
	mkdir build

dir-linux:
	mkdir build/linux

zip-linux:
	zip -rj build/linux/argonaut-$(VERSION).linux.amd64.zip build/linux/argonaut

build-linux:
	GOOS=linux GOARCH=amd64 go build -o build/linux/argonaut

build-windows:
	GOOS=windows GOARCH=amd64 go build -o build/windows/argonaut.exe

dir-windows:
	mkdir build/windows

zip-windows:
	zip -rj build/windows/argonaut-$(VERSION).windows.amd64.zip build/windows/argonaut.exe

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o build/darwin/argonaut

zip-darwin:
	zip -rj build/darwin/argonaut-$(VERSION).darwin.amd64.zip build/darwin/argonaut

dir-darwin:
	mkdir build/darwin

clean:
	rm -r build
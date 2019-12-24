NAME=fargate-td
SHELL=/bin/bash
BUILD_DIR=./build
TARGET_ARCHS=linux/amd64 darwin/amd64
TARGET_DIRS=${subst /,_,${TARGET_ARCHS}}
TARGET_FILES=${TARGET_DIRS:%=${BUILD_DIR}/pkg/${NAME}_%.zip}
GOX_OUTPUT="${BUILD_DIR}/bin/${NAME}_{{.OS}}_{{.Arch}}/{{.Dir}}"

.PHONY: cross-build
cross-build:
	go get github.com/mitchellh/gox
	go get github.com/pwaller/goupx
	gox -osarch="${TARGET_ARCHS}" -ldflags='-s -w -extldflags "-static"' -output=${GOX_OUTPUT} ./cmd/${NAME}
	strip ${BUILD_DIR}/bin/${NAME}_linux_amd64/${NAME}
	goupx ${BUILD_DIR}/bin/${NAME}_linux_amd64/${NAME}

${BUILD_DIR}/pkg/%.zip: cross-build
	mkdir -p ${BUILD_DIR}/pkg
	pushd ${BUILD_DIR}/bin/$* && zip ../../pkg/$*.zip ${NAME} && popd

.PHONY: package
package: ${TARGET_FILES}

.PHONY: release
release: package
	go get github.com/tcnksm/ghr
	ghr ${TAG} ${BUILD_DIR}/pkg

.PHONY: clean
clean:
	rm -rf ${BUILD_DIR}

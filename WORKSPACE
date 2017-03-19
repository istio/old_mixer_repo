workspace(name = "com_github_istio_mixer")

git_repository(
    name = "io_bazel_rules_go",
    commit = "9496d79880a7d55b8e4a96f04688d70a374eaaf4", # Mar 3, 2017 (v0.4.1)
    remote = "https://github.com/bazelbuild/rules_go.git",
)

git_repository(
    name = "org_pubref_rules_protobuf",
    commit = "d42e895387c658eda90276aea018056fcdcb30e4", # Mar 07 2017 (gogo* support)
    remote = "https://github.com/pubref/rules_protobuf",
)

load("//:repositories.bzl", "go_mixer_repositories")
go_mixer_repositories(
    # Change this to True to use ../api directory
    use_local_api = False,
)

new_http_archive(
    name = "docker_ubuntu",
    build_file = "BUILD.ubuntu",
    sha256 = "2c63dd81d714b825acd1cb3629c57d6ee733645479d0fcdf645203c2c35924c5",
    type = "zip",
    url = "https://codeload.github.com/tianon/docker-brew-ubuntu-core/zip/b6f1fe19228e5b6b7aed98dcba02f18088282f90",
)

DEBUG_BASE_IMAGE_SHA="3f57ae2aceef79e4000fb07ec850bbf4bce811e6f81dc8cfd970e16cdf33e622"

# See github.com/istio/manager/blob/master/docker/debug/build-and-publish-debug-image.sh
# for instructions on how to re-build and publish this base image layer.
http_file(
    name = "ubuntu_xenial_debug",
    url = "https://storage.googleapis.com/istio-build/manager/ubuntu_xenial_debug-" + DEBUG_BASE_IMAGE_SHA + ".tar.gz",
    sha256 = DEBUG_BASE_IMAGE_SHA,
)

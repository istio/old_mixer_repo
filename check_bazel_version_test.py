# stub out the native object
class Native(object):

    def __init__(self, v):
        self.bazel_version = v


class Output(object):

    def __init__(self):
        self.output = ""


class TestCase(object):

    def __init__(self, min_ver, max_ver, ver, output):
        self.min_ver = min_ver
        self.max_ver = max_ver
        self.ver = ver
        self.output = output


def test_check_version():
    global native
    global fail
    ok = True
    import shutil
    import os

    shutil.copyfile("check_bazel_version.bzl", "check_bazel_version.py")

    import check_bazel_version as cv

    ts = [TestCase("0.5.3", "0.5.4", "0.6.0", "too new"),
          TestCase("0.5.3", "0.5.4", "0.5.1", "too old"),
          TestCase("0.5.3", "0.5.4", "0.5.1", "")]
    for tc in ts:
        cv.native = Native(tc.ver)
        op = Output()

        def _fail(msg):
            op.output = msg
        cv.fail = _fail

        cv.check_bazel_version(tc.min_ver, tc.max_ver)
        if tc.output == op.output:
            continue

        if tc.output in op.output:
            continue

        print tc, "Got [ ", tc.output, "] want [", op.output
        ok = False

    os.remove("check_bazel_version.py")

    if ok:
        return 0

    return -1

if __name__ == "__main__":
    import sys
    sys.exit(test_check_version())

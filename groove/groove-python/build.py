from distutils.command.build_ext import build_ext
from distutils.core import Distribution
from distutils.errors import (CCompilerError, CompileError, DistutilsExecError,
                              DistutilsPlatformError)
from distutils.extension import Extension
from os import chmod, stat
from pathlib import Path
from shutil import copyfile
from subprocess import run


class GoExtension(Extension):
    def __init__(self, name, path):
        super().__init__(name, sources=[])
        self.path = path


extensions = [
    GoExtension(
        #"groove",
        "groove.assets.grooveproxy",
        # Assume we have temporarily copied over the proxy folder into our current path
        # We don't want it to be referenced in the actual parent library, since we need to bundle
        # it alongside the python project in sdist in case clients need to build from source
        # when wheels aren't available.
        "./proxy",
    )
]


class BuildFailed(Exception):
    pass


class GoExtensionBuilder(build_ext):
    def run(self):
        try:
            build_ext.run(self)
        except (DistutilsPlatformError, FileNotFoundError):
            raise BuildFailed("File not found. Could not compile extension.")

    def build_extension(self, ext):
        try:
            if isinstance(ext, GoExtension):
                extension_root = Path(__file__).parent.resolve() / ext.path
                ext_path = self.get_ext_fullpath(ext.name)
                result = run(["go", "build", "-o", str(Path(ext_path).absolute())], cwd=extension_root)
                if result.returncode != 0:
                    raise CompileError("Go build failed")
            else:
                build_ext.build_extension(self, ext)
        except (CCompilerError, DistutilsExecError, DistutilsPlatformError, ValueError):
            raise BuildFailed('Could not compile C extension.')


def build(setup_kwargs):
    distribution = Distribution({"name": "python_ctypes", "ext_modules": extensions})
    distribution.package_dir = "python_ctypes"

    cmd = GoExtensionBuilder(distribution)
    cmd.ensure_finalized()
    cmd.run()

    # This is somewhat of a hack with go executables; this pipeline will package
    # them as .so files but they aren't actually built libraries. We maintain
    # this convention only for the ease of plugging in to poetry and distutils that
    # use this suffix to indicate the build architecture and run on the
    # correct downstream client OS.
    for output in cmd.get_outputs():
        relative_extension = Path(output).relative_to(cmd.build_lib)
        copyfile(output, relative_extension)
        mode = stat(relative_extension).st_mode
        mode |= (mode & 0o444) >> 2
        chmod(relative_extension, mode)


if __name__ == "__main__":
    build({})

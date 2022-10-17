from signal import SIGTERM
from subprocess import Popen

from psutil import Process as PsutilProcess


def terminate_all(process: Popen):
    """
    The normal `.terminate` just kills the shell. If subprocesses have been
    spawned from the shell (which is normal within our go processes) then these
    won't be cleaned up and will be left hanging.

    This function terminates all spawned subprocesses.

    """
    if process.returncode is not None:
        # No-op if process has already exited
        return

    signal = SIGTERM
    process = PsutilProcess(process.pid)
    for child in process.children(recursive=True):
        child.send_signal(signal)
    process.send_signal(signal)

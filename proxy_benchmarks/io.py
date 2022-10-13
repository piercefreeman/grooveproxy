from pathlib import Path

def is_docker() -> bool:
    """
    Check if we are running inside a Docker container.

    """
    cgroup_path = Path("/proc/self/cgroup")
    if not cgroup_path.exists():
        return False
    with open("/proc/self/cgroup", 'r') as f:
        return "docker" in f.read()


def wrap_command_with_sudo(command: list[str]):
    """
    Depending on the environment we have different needs for sudo permissions. When
    running on a local machine, we can use sudo to elevate permissions. When running
    in a docker container, we can't prompt for a password, so we expect that the process
    running the command is in the appropriate process group.

    """
    if is_docker():
        return command
    return ["sudo", *command]

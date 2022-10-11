from dataclasses import dataclass
from os import environ
from subprocess import Popen

from configargparse import DefaultConfigFileParser

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.process import terminate_all


@dataclass
class LoadTestResults:
    stats: str
    exceptions: str
    failures: str
    stats_history: str


def load_config(path: str) -> str:
    config = DefaultConfigFileParser()
    with open(path) as file:
        return config.parse(file)


def run_load_test(
    config_path: str,
    run_time_seconds: int = 60,
    spawn_processes: int = 5,
    proxy_port: int | None = None
) -> LoadTestResults:
    server_path = get_asset_path("speed-test/server")
    locust_project_path = get_asset_path("speed-test/locust")
    locust_config = load_config(locust_project_path / config_path)

    env = environ.copy()

    if proxy_port:
        env["PROXY_PORT"] = str(proxy_port)

    try:
        server_process = Popen("go run .", shell=True, cwd=server_path)

        # Launch the coordination server
        # This will wait to launch until N processes have connected
        main_process = Popen(
            f"poetry run locust --run-time {run_time_seconds}s --master --expect-workers {spawn_processes} --config={config_path}",
            shell=True,
            cwd=locust_project_path,
            env=env
        )

        worker_processes = [
            Popen(
                f"poetry run locust --worker --config={config_path}",
                shell=True,
                cwd=locust_project_path,
                env=env
            )
            for _ in range(spawn_processes)
        ]

        main_process.wait()

    finally:
        terminate_all(server_process)
        terminate_all(main_process)
        for process in worker_processes:
            terminate_all(process)

    # Path prefix to the csv files, relative to the locust project
    csv_prefix = locust_config["csv"]

    return LoadTestResults(
        stats=locust_project_path / f"{csv_prefix}_stats.csv",
        stats_history=locust_project_path / f"{csv_prefix}_stats_history.csv",
        exceptions=locust_project_path / f"{csv_prefix}_exceptions.csv",
        failures=locust_project_path / f"{csv_prefix}_failures.csv",
    )

from os import getenv

from locust import (
    FastHttpUser,
    HttpUser,
    events,
    task,
)
from locust.runners import MasterRunner, WorkerRunner


@events.init.add_listener
def on_locust_init(environment, **_kwargs):
    # Fix issue on 
    if isinstance(environment.runner, MasterRunner):
        environment.stats.use_response_times_cache = True
    if isinstance(environment.runner, WorkerRunner):
        environment.stats.use_response_times_cache = True


load_test_certificate = getenv("LOAD_TEST_CERTIFICATE")
load_test_certificate_key = getenv("LOAD_TEST_CERTIFICATE_KEY")


#class WebsiteUser(FastHttpUser):
class WebsiteUser(HttpUser):
    @task
    def index(self):
        self.client.get(
            "/handle",
            verify=load_test_certificate,
        )

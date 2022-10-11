from os import getenv

from locust import FastHttpUser, HttpUser, events, task
from locust.runners import MasterRunner, WorkerRunner


@events.init.add_listener
def on_locust_init(environment, **_kwargs):
    # Fix issue on 
    if isinstance(environment.runner, MasterRunner):
        environment.stats.use_response_times_cache = True
    if isinstance(environment.runner, WorkerRunner):
        environment.stats.use_response_times_cache = True

proxy_port = getenv("PROXY_PORT")
if not proxy_port:
    raise ValueError("Proxy port is required.")

proxies = {
   "http": f"http://localhost:{proxy_port}",
   "https": f"http://localhost:{proxy_port}",
}

print("Proxy configuration", proxies)

class WebsiteUser(HttpUser):
    @task
    def index(self):
        self.client.get("/handle", proxies=proxies, verify=False)

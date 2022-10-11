from os import getenv

from locust import FastHttpUser, HttpUser, events, task
from locust.runners import MasterRunner, WorkerRunner
#from warnings import filterwarnings


#filterwarnings("ignore", message="Unverified HTTPS request")


@events.init.add_listener
def on_locust_init(environment, **_kwargs):
    # Fix issue on 
    if isinstance(environment.runner, MasterRunner):
        environment.stats.use_response_times_cache = True
    if isinstance(environment.runner, WorkerRunner):
        environment.stats.use_response_times_cache = True

proxy_port = getenv("PROXY_PORT")
proxy_certificate = getenv("PROXY_CERTIFICATE")
proxy_certificate_key = getenv("PROXY_CERTIFICATE_KEY")

if not proxy_port:
    raise ValueError("Proxy port is required.")
if not proxy_certificate:
    raise ValueError("Proxy certificate is required.")
if not proxy_certificate_key:
    raise ValueError("Proxy certificate key is required.")

proxies = {
   "http": f"http://localhost:{proxy_port}",
   "https": f"http://localhost:{proxy_port}",
}

print("Proxy configuration", proxies)

class WebsiteUser(HttpUser):
    @task
    def index(self):
        self.client.get(
            "/handle",
            proxies=proxies,
            verify=False,
            #cert=(
            #    proxy_certificate,
            #    proxy_certificate_key
            #),
        )

from locust import HttpUser, task, FastHttpUser
from locust import events
from locust.runners import MasterRunner, WorkerRunner

@events.init.add_listener
def on_locust_init(environment, **_kwargs):
    # Fix issue on 
    if isinstance(environment.runner, MasterRunner):
        environment.stats.use_response_times_cache = True
    if isinstance(environment.runner, WorkerRunner):
        environment.stats.use_response_times_cache = True


#class WebsiteUser(HttpUser):
class WebsiteUser(FastHttpUser):
    @task
    def index(self):
        self.client.get("/handle")

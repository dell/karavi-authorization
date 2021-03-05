import time, requests
from locust import HttpUser, TaskSet, task

TOKENS = None

class TenantCreateAndDeleteVolumeTask(TaskSet):
    wait_time = between(1, 2.5)
    token = ""

    @task
    def create_volume(self):
        headers = {'Authorization': 'Bearer ' + self.token, 'Forwarded': 'by=vxflexos,for=https://10.247.66.155:8000;7045c4cc20dffc0f'}
        start_time = time.time()
        try:
            with self.client.get("/api/types/StoragePool/instances/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)
            
            payload = {"volumeSizeInKb":"100", "storagePoolId":"3df6b86600000000"}
            with self.client.post("/api/types/Volume/instances/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/types/StoragePool/instances/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/types/StoragePool/instances/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            payload = {"volumeSizeInKb":"100", "storagePoolId":"3df6b86600000000"}
            with self.client.post("/api/types/Volume/instances/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            payload = {"name":"TestVolume"}
            with self.client.post("/api/types/Volume/instances/action/queryIdByKey/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                    if resp.status_code > 400:
                        response.failure(resp.text)

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            payload = {"sdcId":"ce3356fe00000003","allowMultipleMappings":"TRUE","accessMode":"ReadWrite"}
            with self.client.post("/api/instances/Volume::000000000000001/action/addMappedSdc/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            total_time = int((time.time() - start_time) * 1000)
            events.request_success.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0)
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000)
            events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)

    @task
    def delete_volume(self):
        headers = {'Authorization': 'Bearer ' + self.token, 'Forwarded': 'by=vxflexos,for=https://10.247.66.155:8000;7045c4cc20dffc0f'}
        start_time = time.time()
        try:
            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)                    

            payload = {{"sdcId":"ce3356fe00000003", "skipApplianceValidation":"TRUE"}}
            with self.client.get("/api/instances/Volume::000000000000001/action/removeMappedSdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text) 

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)   

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)     

            payload = {"removeMode":"ONLY_ME"}
            with self.client.get("/api/instances/Volume::000000000000001/action/removeVolume/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)   

            with self.client.get("/api/instances/Volume::000000000000001/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    response.failure(resp.text)                                                           

            total_time = int((time.time() - start_time) * 1000)
            events.request_success.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0)
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000)
            events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)

    def on_start(self):
        self.client.verify = False
        if len(TOKENS) > 0:
            self.token = TOKENS.pop()

class TenantCreateAndDeleteVolume(HttpUser):
    tasks = [TenantCreateAndDeleteVolumeTask]
    host = "https://10.247.66.155"
    sock = None

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        global TOKENS
        if (TOKENS == None):
            with open('tokens.txt') as f:
                TOKENS = f.read().splitlines()
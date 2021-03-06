import time, requests, uuid
from locust import HttpUser, TaskSet, task

TOKENS = None

def setTokens():
    global TOKENS
    if (TOKENS == None):
        with open('tokens.txt') as f:
            TOKENS = f.read().splitlines()

setTokens()

class TenantCreateAndDeleteVolumeTask(TaskSet):
    wait_time = between(1, 2.5)
    token = ""

    @task
    def create_volume(self):
        headers = {'Authorization': 'Bearer ' + self.token, 'Forwarded': 'by=vxflexos,for=https://10.247.66.155:8000;7045c4cc20dffc0f'}

        # create volume
        volNameUUID = str(uuid.uuid4())
        volName = volNameUUID.replace('-', '')
        start_time = time.time()
        try:
            with self.client.get("/api/types/StoragePool/instances/", headers=headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
            
            payload = {"volumeSizeInKb":"100", "storagePoolId":"dcc71b0500000000", "name":volName}
            with self.client.post("/api/types/Volume/instances/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            with self.client.get("/api/types/StoragePool/instances/", headers=headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            with self.client.get("/api/types/StoragePool/instances/", headers=headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            payload = {"volumeSizeInKb":"100", "storagePoolId":"dcc71b0500000000"}
            with self.client.post("/api/types/Volume/instances/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            payload = {"name":"TestVolume"}
            with self.client.post("/api/types/Volume/instances/action/queryIdByKey/", headers=headers, json=payload, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                    if resp.status_code >= 400:
                        resp.failure(resp.text)

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            payload = {"sdcId":"34e49c2000000004","allowMultipleMappings":"TRUE","accessMode":"ReadWrite"}
            with self.client.post("/api/instances/Volume::" + volName + "/action/addMappedSdc/", headers=headers, json=payload, catch_response=True, name="/api/instances/Volume::<id>/action/addMappedSdc/") as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)

            total_time = int((time.time() - start_time) * 1000)
            events.request_success.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0)
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000)
            events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)

        # delete volume
        start_time = time.time()
        try:
            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)                    

            payload = {"sdcId":"34e49c2000000004", "skipApplianceValidation":"TRUE"}
            with self.client.post("/api/instances/Volume::" + volName + "/action/removeMappedSdc/", headers=headers, json=payload, catch_response=True, name="/api/instances/Volume::<id>/action/addMappedSdc/") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text) 

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)   

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)     

            payload = {"removeMode":"ONLY_ME"}
            with self.client.post("/api/instances/Volume::" + volName + "/action/removeVolume/", headers=headers, json=payload, catch_response=True, name="/api/instances/Volume::<id>/action/removeVolume/") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)   

            with self.client.get("/api/instances/Volume::" + volName + "/", headers=headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)                                                           

            total_time = int((time.time() - start_time) * 1000)
            events.request_success.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0)
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000)
            events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)

    def on_start(self):
        self.client.verify = False
        if len(TOKENS) > 0:
            self.token = TOKENS.pop()

class TenantCreateAndDeleteVolume(HttpUser):
    tasks = [TenantCreateAndDeleteVolumeTask]
    host = "https://10.247.66.155"
    sock = None

    #def __init__(self, *args, **kwargs):
        #super().__init__(*args, **kwargs)
        #global TOKENS
        #if (TOKENS == None):
            #with open('tokens.txt') as f:
                #TOKENS = f.read().splitlines()
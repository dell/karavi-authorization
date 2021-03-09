import time, requests, uuid
from locust import HttpUser, TaskSet, task

TOKENS = None

def setTokens():
    global TOKENS
    if (TOKENS == None):
        with open('tokens.txt') as f:
            TOKENS = f.read().splitlines()

setTokens()

class TenantCreateAndDeleteVolumeTask(SequentialTaskSet):
    wait_time = between(1, 2.5)
    token = ""
    volName = ""
    headers = {}

    @task
    def create_volume(self):
        start_time = time.time()
        try:
            with self.client.get("/api/types/StoragePool/instances/", headers=self.headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 
            
            payload = {"volumeSizeInKb":"100", "storagePoolId":"dcc71b0500000000"}
            with self.client.post("/api/types/Volume/instances/", headers=self.headers, json=payload, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 

            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return

            with self.client.get("/api/types/StoragePool/instances/", headers=self.headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 

            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 

            with self.client.get("/api/types/StoragePool/instances/", headers=self.headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 

            payload = {"volumeSizeInKb":"100", "storagePoolId":"dcc71b0500000000"}
            with self.client.post("/api/types/Volume/instances/", headers=self.headers, json=payload, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 

            payload = {"name": self.volName}
            with self.client.post("/api/types/Volume/instances/action/queryIdByKey/", headers=self.headers, json=payload, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return 

            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                    if resp.status_code >= 400:
                        resp.failure(resp.text)
                        events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                        return 

            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=self.headers, catch_response=True) as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return  

            payload = {"sdcId":"34e49c2000000004","allowMultipleMappings":"TRUE","accessMode":"ReadWrite"}
            with self.client.post("/api/instances/Volume::" + self.volName + "/action/addMappedSdc/", headers=self.headers, json=payload, catch_response=True, name="/api/instances/Volume::<id>/action/addMappedSdc/") as resp:
                if resp.status_code >= 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)
                    return

            total_time = int((time.time() - start_time) * 1000)
            events.request_success.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0)
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000)
            events.request_failure.fire(request_type="task", name="create_volume", response_time=total_time, response_length=0, exception=e)    


    
    @task
    def delete_volume(self):
        start_time = time.time()
        try:
            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return

            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=self.headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)                    
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            payload = {"sdcId":"34e49c2000000004", "skipApplianceValidation":"TRUE"}
            with self.client.post("/api/instances/Volume::" + self.volName + "/action/removeMappedSdc/", headers=self.headers, json=payload, catch_response=True, name="/api/instances/Volume::<id>/action/removeMappedSdc/") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text) 
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)   
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            with self.client.get("/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/", headers=self.headers, catch_response=True) as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)     
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            payload = {"removeMode":"ONLY_ME"}
            with self.client.post("/api/instances/Volume::" + self.volName + "/action/removeVolume/", headers=self.headers, json=payload, catch_response=True, name="/api/instances/Volume::<id>/action/removeVolume/") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)   
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            with self.client.get("/api/instances/Volume::" + self.volName + "/", headers=self.headers, catch_response=True, name="/api/instances/Volume::<id>") as resp:
                if resp.status_code > 400:
                    resp.failure(resp.text)                                                           
                    events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e)
                    return
 
            total_time = int((time.time() - start_time) * 1000)
            events.request_success.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0)
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000)
            events.request_failure.fire(request_type="task", name="delete_volume", response_time=total_time, response_length=0, exception=e) 

    def on_start(self):
        self.client.verify = False
        if len(TOKENS) > 0:
            self.token = TOKENS.pop()
        volNameUUID = str(uuid.uuid4())
        self.volName = volNameUUID.replace('-', '')
        self.headers = {'Authorization': 'Bearer ' + self.token, 'x-csi-pv-name': self.volName, 'Forwarded': 'by=vxflexos,for=https://10.247.66.155:8000;7045c4cc20dffc0f'}

class TenantCreateAndDeleteVolume(HttpUser):
    tasks = [TenantCreateAndDeleteVolumeTask]
    sock = None

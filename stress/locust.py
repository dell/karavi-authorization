import time
from locust import HttpUser, task, between

class TenantA(HttpUser):
    wait_time = between(1, 2.5)

    @task
    def create_volume(self):
        payload = {"volumeSizeInKb":"100", "storagePoolId":"3df6b86600000000", "name": "TenantAVol"}
        headers = {'Forwarded':'by=csi-vxflexos,for=https://10.247.66.155:8000;7045c4cc20dffc0f'}
        resp = self.client.post("/api/types/Volume/instances/", json=payload, headers=headers)
        print("Response text:", resp.text)

    def on_start(self):
        self.client.verify = False

class TenantB(HttpUser):
    wait_time = between(1, 2.5)

    @task
    def create_volume(self):
        payload = {"volumeSizeInKb":"100", "storagePoolId":"3df6b86600000000", "name": "TenantBVol"}
        headers = {'Forwarded':'by=csi-vxflexos,for=https://10.247.66.155:8000;7045c4cc20dffc0f'}
        resp = self.client.post("/api/types/Volume/instances/", json=payload, headers=headers)
        print("Response text:", resp.text)

    def on_start(self):
        self.client.verify = False

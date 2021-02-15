package karavi.authz.url

test_get_api_login_allowed {
    allow with input as {"method": "GET", "url": "/api/login/"}
}

test_post_proxy_refresh_token_allowed {
    allow with input as {"method": "POST", "url": "/proxy/refresh-token/"}
}

test_get_api_version_allowed {
    allow with input as {"method": "GET", "url": "/api/version/"}
}

test_get_system_instances_allowed {
    allow with input as {"method": "GET", "url": "/api/types/System/instances/"}
}

test_get_storagpool_instances_allowed {
    allow with input as {"method": "GET", "url": "/api/types/StoragePool/instances/"}
}

test_post_volume_instances_allowed {
    allow with input as {"method": "POST", "url": "/api/types/Volume/instances/"}
}

test_get_volume_instance_allowed {
    allow with input as {"method": "GET", "url": "/api/instances/Volume::2a3814c600000003/"}
}

test_post_volume_instances_queryIdByKey_allowed {
    allow with input as {"method": "POST", "url": "/api/types/Volume/instances/action/queryIdByKey/"}
}

test_get_system_sdc_allowed {
    allow with input as {"method": "GET", "url": "/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/"}
}

test_post_volume_add_sdc_allowed {
    allow with input as {"method": "POST", "url": "/api/instances/Volume::2a3814c600000003/action/addMappedSdc/"}
}

test_post_volume_remove_sdc_allowed {
    allow with input as {"method": "POST", "url": "/api/instances/Volume::2a3814c600000003/action/removeMappedSdc/"}
}

test_post_volume_remove_allowed {
    allow with input as {"method": "POST", "url": "/api/instances/Volume::2a3814c600000003/action/removeVolume/"}
}
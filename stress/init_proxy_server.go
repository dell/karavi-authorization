package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

const (
	sharedSecret = "secret"
	role         = "CSIBronze"
)

var (
	numTenants = 500
	endpoint   = "https://10.0.0.1"
)

func main() {
	flag.IntVar(&numTenants, "-t", 500, "")
	flag.StringVar(&endpoint, "-e", "https://10.0.0.1", "")
	flag.Parse()

	resp := karavictlStorageCreate(endpoint)
	if resp.err != nil {
		fmt.Printf("error creating storage: %s", string(resp.out))
	}

	resp = karavictlRoleCreate()
	if resp.err != nil {
		fmt.Printf("error creating role: %s", string(resp.out))
	}

	// open tokens.txt and delete contents
	file, err := os.OpenFile("tokens.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	for i := 0; i < numTenants; i++ {
		// generate a tenant name ("Group" field in the token)
		tenant := uuid.New().String()

		// create the tenant in proxy server
		resp := karavictlTenantCreate(tenant)
		if resp.err != nil {
			fmt.Printf("error creating tenant %s: %s", tenant, string(resp.out))
		}

		// bind the tenant to the role in the proxy server
		resp = karavictlBindTenantToRole(tenant, role)
		if resp.err != nil {
			fmt.Printf("error binding role %s to tenant %s: %s", role, tenant, string(resp.out))
		}

		// generate a token for the tenant and save to tokens.txt
		err := createTokenAndWriteToFile(file, tenant)
		if err != nil {
			fmt.Printf("error writing token to file: %+v", err)
		}
	}
}

func createTokenAndWriteToFile(f *os.File, tenant string) error {
	claims := struct {
		jwt.StandardClaims
		Role  string `json:"role"`
		Group string `json:"group"`
	}{
		StandardClaims: jwt.StandardClaims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: 1914886001,
			Audience:  "karavi",
			Subject:   "karavi-tenant",
		},
		Role:  role,
		Group: tenant,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(sharedSecret))
	if err != nil {
		return fmt.Errorf("failed to sign token for tenant %s: %+v", tenant, err)
	}

	_, err = f.WriteString(accessToken + "\n")
	if err != nil {
		return fmt.Errorf("failed to write token to file: %+v", err)
	}
	f.Sync()
	return nil
}

type cmdResp struct {
	err error
	out []byte
}

func karavictlStorageCreate(endpoint string) cmdResp {
	out, err := exec.Command("karavictl", "storage", "create",
		"-e", endpoint,
		"-u", "admin",
		"-p", "Password123",
		"-s", "7045c4cc20dffc0f",
		"-t", "powerflex",
		"-i").CombinedOutput()
	return cmdResp{
		err: err,
		out: out,
	}
}

func karavictlRoleCreate() cmdResp {
	out, err := exec.Command("karavictl", "role", "create",
		"--from-file", "roles.json").CombinedOutput()
	return cmdResp{
		err: err,
		out: out,
	}
}

func karavictlTenantCreate(tenant string) cmdResp {
	out, err := exec.Command("karavictl", "tenant", "create", "-n", tenant).CombinedOutput()
	return cmdResp{
		err: err,
		out: out,
	}
}

func karavictlBindTenantToRole(tenant, role string) cmdResp {
	out, err := exec.Command("karavictl", "rolebinding", "create", "--tenant", tenant, "--role", role).CombinedOutput()
	return cmdResp{
		err: err,
		out: out,
	}
}

func karavictlTenantList() cmdResp {
	out, err := exec.Command("karavictl", "tenant", "list").CombinedOutput()
	return cmdResp{
		err: err,
		out: out,
	}
}

func karavictlTenantDelete(tenant string) cmdResp {
	out, err := exec.Command("karavictl", "tenant", "delete", "-n", tenant).CombinedOutput()
	return cmdResp{
		err: err,
		out: out,
	}
}

func deleteAllTenants() error {
	tenants := struct {
		Tenants []struct {
			Name string `json:"name"`
		} `json:"tenants"`
	}{}

	resp := karavictlTenantList()
	if resp.err != nil {
		fmt.Println(string(resp.out))
		return resp.err
	}

	err := json.Unmarshal(resp.out, &tenants)
	if err != nil {
		return err
	}

	for _, t := range tenants.Tenants {
		resp := karavictlTenantDelete(t.Name)
		if resp.err != nil {
			fmt.Println(string(resp.out))
			return resp.err
		}
	}

	return nil
}

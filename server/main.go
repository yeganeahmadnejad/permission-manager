package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sighupio/permission-manager/server/kube"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var kubeclient *kubernetes.Clientset

func main() {
	kubeclient = kube.NewKubeclient()

	e := echo.New()

	// e.Use(middleware.Logger())
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.GET("/api/list-users", listUsers)
	e.GET("/api/list-groups", listGroups)

	e.GET("/api/list-namespace", listNamespaces)

	e.GET("/api/rbac", listRbac)

	e.POST("/api/create-cluster-role", createClusterRole)
	e.POST("/api/create-user", createUser)
	e.POST("/api/create-rolebinding", createRolebinding)
	e.POST("/api/create-cluster-rolebinding", createClusterRolebinding)

	e.POST("/api/delete-cluster-role", deleteClusterRole)
	e.POST("/api/delete-cluster-rolebinding", deleteClusterRolebinding)
	e.POST("/api/delete-rolebinding", deleteRolebinding)
	e.POST("/api/delete-role", deleteRole)

	e.POST("/api/create-kubeconfig", createKubeconfig)

	e.Logger.Fatal(e.Start(":4000"))
}

type user struct {
	Name string `json:"name"`
}

var users []user

func listUsers(c echo.Context) error {
	return c.JSON(http.StatusOK, []user{
		user{Name: "jaga"},
	})
}

func createUser(c echo.Context) error {
	type Request struct {
		Name string `json:"name"`
	}
	r := new(Request)
	if err := c.Bind(r); err != nil {
		panic(err)
		return err
	}

	users = append(users, user{Name: r.Name})

	return c.JSON(http.StatusOK, r)
}

func listGroups(c echo.Context) error {
	type Group struct {
		Name string `json:"name"`
	}
	return c.JSON(http.StatusOK, []Group{})
}

func listNamespaces(c echo.Context) error {
	namespaces, err := kubeclient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	type Response struct {
		Namespaces []v1.Namespace `json:"namespaces"`
	}

	return c.JSON(http.StatusOK, Response{
		Namespaces: namespaces.Items,
	})
}

func listRbac(c echo.Context) error {
	type Response struct {
		ClusterRoles        []rbacv1.ClusterRole        `json:"clusterRoles"`
		ClusterRoleBindings []rbacv1.ClusterRoleBinding `json:"clusterRoleBindings"`
		Roles               []rbacv1.Role               `json:"roles"`
		RoleBindings        []rbacv1.RoleBinding        `json:"roleBindinds"`
	}

	clusterRoles, err := kubeclient.RbacV1().ClusterRoles().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	clusterRoleBindings, err := kubeclient.RbacV1().ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	roles, err := kubeclient.RbacV1().Roles("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	roleBindings, err := kubeclient.RbacV1().RoleBindings("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	return c.JSON(http.StatusOK, Response{
		ClusterRoles:        clusterRoles.Items,
		ClusterRoleBindings: clusterRoleBindings.Items,
		Roles:               roles.Items,
		RoleBindings:        roleBindings.Items,
	})
}

func createClusterRole(c echo.Context) error {
	type Request struct {
		RoleName string              `json:"roleName"`
		Rules    []rbacv1.PolicyRule `json:"rules"`
	}
	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	type Response struct {
		Ok bool `json:"ok"`
	}

	kubeclient.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.RoleName,
		},
		Rules: r.Rules,
	})

	return c.JSON(http.StatusOK, Response{Ok: true})
}

func createRolebinding(c echo.Context) error {
	type Request struct {
		RolebindingName string           `json:"rolebindingName"`
		Namespace       string           `json:"namespace"`
		Username        string           `json:"user"`
		Subjects        []rbacv1.Subject `json:"subjects"`
		RoleKind        string           `json:"roleKind"`
		RoleName        string           `json:"roleName"`
	}
	r := new(Request)
	if err := c.Bind(r); err != nil {
		panic(err)
		return err
	}

	type Response struct {
		Ok bool `json:"ok"`
	}

	kubeclient.RbacV1().RoleBindings(r.Namespace).Create(&rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.RolebindingName,
			Namespace: r.Namespace,
			Labels:    map[string]string{"generated_for_user": r.Username},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     r.RoleKind,
			Name:     r.RoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: r.Subjects,
	})

	return c.JSON(http.StatusOK, Response{Ok: true})
}

func createClusterRolebinding(c echo.Context) error {
	type Request struct {
		ClusterRolebindingName string           `json:"clusterRolebindingName"`
		Username               string           `json:"user"`
		Subjects               []rbacv1.Subject `json:"subjects"`
		RoleName               string           `json:"roleName"`
	}
	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	type Response struct {
		Ok bool `json:"ok"`
	}

	kubeclient.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   r.ClusterRolebindingName,
			Labels: map[string]string{"generated_for_user": r.Username},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     r.RoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: r.Subjects,
	})

	return c.JSON(http.StatusOK, Response{Ok: true})
}

func deleteClusterRole(c echo.Context) error {
	type Request struct {
		RoleName string `json:"roleName"`
	}
	type Response struct {
		Ok bool `json:"ok"`
	}

	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	kubeclient.RbacV1().ClusterRoles().Delete(r.RoleName, nil)
	return c.JSON(http.StatusOK, Response{Ok: true})
}

func deleteClusterRolebinding(c echo.Context) error {
	type Request struct {
		RolebindingName string `json:"rolebindingName"`
	}
	type Response struct {
		Ok bool `json:"ok"`
	}

	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	kubeclient.RbacV1().ClusterRoleBindings().Delete(r.RolebindingName, nil)
	return c.JSON(http.StatusOK, Response{Ok: true})
}

func deleteRole(c echo.Context) error {
	type Request struct {
		RoleName  string `json:"roleName"`
		Namespace string `json:"namespace"`
	}
	type Response struct {
		Ok bool `json:"ok"`
	}

	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	kubeclient.RbacV1().Roles(r.Namespace).Delete(r.RoleName, nil)
	return c.JSON(http.StatusOK, Response{Ok: true})
}

func deleteRolebinding(c echo.Context) error {
	type Request struct {
		RolebindingName string `json:"rolebindingName"`
		Namespace       string `json:"namespace"`
	}
	type Response struct {
		Ok bool `json:"ok"`
	}

	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	kubeclient.RbacV1().RoleBindings(r.Namespace).Delete(r.RolebindingName, nil)
	return c.JSON(http.StatusOK, Response{Ok: true})
}

func createKubeconfig(c echo.Context) error {
	type Request struct {
		Username string `json:"username"`
	}
	r := new(Request)
	if err := c.Bind(r); err != nil {
		return err
	}

	rsaFile, err := ioutil.TempFile(os.TempDir(), "prefix-")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(rsaFile.Name())

	rsaPrivateKey, err := exec.Command("openssl", "genrsa", "4096").Output()
	if err != nil {
		panic(err)
	}

	if _, err = rsaFile.Write(rsaPrivateKey); err != nil {
		log.Fatal("Failed to write to temporary file", err)
	}

	subj := fmt.Sprintf("/CN=%s", r.Username)
	cmd := exec.Command("openssl", "req", "-new", "-key", rsaFile.Name(), "-subj", subj)
	csr, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprint(err))
	}

	clientCsrFile, err := ioutil.TempFile(os.TempDir(), "prefix-")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	if _, err = clientCsrFile.Write(csr); err != nil {
		log.Fatal("Failed to write to temporary file", err)
	}
	defer os.Remove(clientCsrFile.Name())
	crt, err := exec.Command("openssl", "x509", "-req", "-days", "365", "-sha256",
		"-in",
		clientCsrFile.Name(),
		"-CA",
		filepath.Join(os.Getenv("HOME"), ".minikube", "ca.crt"),
		"-CAkey",
		filepath.Join(os.Getenv("HOME"), ".minikube", "ca.key"),
		"-set_serial",
		"2",
	).Output()
	if err != nil {
		panic(err)
	}

	clusterName := "minikube"
	cacertPath := filepath.Join(os.Getenv("HOME"), ".minikube", "ca.crt")

	crtBase64 := base64.StdEncoding.EncodeToString(crt)
	rsaPrivateKeyBase64 := base64.StdEncoding.EncodeToString(rsaPrivateKey)

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
preferences:
    colors: true
current-context: %s
clusters:
  - name: %s
    cluster:
      server: https://192.168.99.100:8443
      certificate-authority: %s}
contexts:
  - context:
      cluster: %s
      user: %s
    name: %s
users:
  - name: %s
    user:
      client-certificate-data: %s
      client-key-data: %s`,
		clusterName, clusterName, cacertPath, clusterName, r.Username, clusterName, r.Username, crtBase64, rsaPrivateKeyBase64)

	type Response struct {
		Ok         bool   `json:"ok"`
		Kubeconfig string `json:"kubeconfig"`
	}
	return c.JSON(http.StatusOK, Response{Ok: true, Kubeconfig: kubeconfig})
}
package iapProvider

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "strings"

    "github.com/hexa-org/policy-mapper/api/policyprovider"
    appengine2 "google.golang.org/api/appengine/v1beta"
    "google.golang.org/api/iam/v1"
)

type HTTPClient interface {
    Get(url string) (resp *http.Response, err error)
    Post(url, contentType string, body io.Reader) (resp *http.Response, err error)
}

type GoogleClient struct {
    HttpClient HTTPClient
    ProjectId  string
}

type backends struct {
    ID        string        `json:"id"`
    Resources []backendInfo `json:"items"`
}

type backendInfo struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

func (c *GoogleClient) GetAppEngineApplications() ([]policyprovider.ApplicationInfo, error) {
    url := fmt.Sprintf("https://appengine.googleapis.com/v1beta/apps/%s", c.ProjectId)
    var appEngine appengine2.Application

    get, err := c.HttpClient.Get(url)
    if err != nil {
        log.Println("Unable to find google cloud app engine applications.")
        return []policyprovider.ApplicationInfo{}, err
    }

    if get.StatusCode == 404 {
        log.Println("No App Engine Found")
        return []policyprovider.ApplicationInfo{}, nil
    }

    log.Printf("Google cloud response %s.\n", get.Status)

    if err = json.NewDecoder(get.Body).Decode(&appEngine); err != nil {
        log.Println("Unable to decode google cloud app engine applications.")
        return []policyprovider.ApplicationInfo{}, err
    }

    log.Printf("Found google cloud backend app engine applications %s.\n", appEngine.Name)

    apps := []policyprovider.ApplicationInfo{
        {ObjectID: appEngine.Id, Name: appEngine.Name, Description: appEngine.DefaultHostname, Service: "AppEngine"},
    }
    return apps, nil
}

func (c *GoogleClient) GetBackendApplications() ([]policyprovider.ApplicationInfo, error) {
    url := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/global/backendServices", c.ProjectId)

    get, err := c.HttpClient.Get(url)
    if err != nil {
        log.Println("Unable to find google cloud backend services.")
        return []policyprovider.ApplicationInfo{}, err
    }
    log.Printf("Google cloud response %s.\n", get.Status)

    var backend backends
    if err = json.NewDecoder(get.Body).Decode(&backend); err != nil {
        log.Println("Unable to decode google cloud backend services.")
        return []policyprovider.ApplicationInfo{}, err
    }

    var apps []policyprovider.ApplicationInfo
    for _, info := range backend.Resources {
        log.Printf("Found google cloud backend services %s.\n", info.Name)
        var service string
        if strings.HasPrefix(info.Name, "k8s") {
            service = "Kubernetes"
        } else {
            service = "Cloud Run"
        }
        apps = append(apps, policyprovider.ApplicationInfo{ObjectID: info.ID, Name: info.Name, Description: info.Description, Service: service})
    }
    return apps, nil
}

type policy struct {
    Policy bindings `json:"policy"`
}

type bindings struct {
    Bindings []iam.Binding `json:"bindings"`
}

func (c *GoogleClient) GetBackendPolicy(name, objectId string) ([]iam.Binding, error) {
    var url string
    if strings.HasPrefix(name, "apps") { // todo - revisit and improve the decision here
        url = fmt.Sprintf("https://iap.googleapis.com/v1/projects/%s/iap_web/appengine-%s/services/default:getIamPolicy", c.ProjectId, objectId)
    } else {
        url = fmt.Sprintf("https://iap.googleapis.com/v1/projects/%s/iap_web/compute/services/%s:getIamPolicy", c.ProjectId, objectId)
    }

    post, err := c.HttpClient.Post(url, "application/json", bytes.NewReader([]byte{}))
    if err != nil {
        log.Println("Unable to find google cloud policy.")
        return []iam.Binding{}, err
    }
    log.Printf("Google cloud response %s.\n", post.Status)

    var bindings bindings
    if err = json.NewDecoder(post.Body).Decode(&bindings); err != nil {
        log.Println("Unable to decode google cloud policy.")
        return []iam.Binding{}, err
    }

    return bindings.Bindings, nil

}

func (c *GoogleClient) SetBackendPolicy(name, objectId string, binding *iam.Binding) error { // todo - objectId may no longer be needed, at least for google
    var url string
    if strings.HasPrefix(name, "apps") { // todo - revisit and improve the decision here
        url = fmt.Sprintf("https://iap.googleapis.com/v1/projects/%s/iap_web/appengine-%s/services/default:setIamPolicy", c.ProjectId, objectId)
    } else {
        url = fmt.Sprintf("https://iap.googleapis.com/v1/projects/%s/iap_web/compute/services/%s:setIamPolicy", c.ProjectId, objectId)
    }

    setPolicy := policy{Policy: bindings{Bindings: []iam.Binding{*binding}}}

    b := new(bytes.Buffer)
    _ = json.NewEncoder(b).Encode(setPolicy)

    _, err := c.HttpClient.Post(url, "application/json", b)
    return err
}

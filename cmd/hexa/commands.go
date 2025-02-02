// This code based on contributions from https://github.com/i2-open/i2goSignals with permission
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/hexa-org/policy-mapper/api/policyprovider"
	"github.com/hexa-org/policy-mapper/models/formats/cedar"
	"github.com/hexa-org/policy-mapper/models/formats/gcpBind"
	"github.com/hexa-org/policy-mapper/pkg/hexapolicy"
	"github.com/hexa-org/policy-mapper/pkg/hexapolicysupport"
	"github.com/hexa-org/policy-mapper/sdk"
	"golang.org/x/oauth2/clientcredentials"
)

var MapFormats = []string{"gcp", "cedar"}

var seperatorline = "==============================================================================="

func openIntegration(alias string, options ...func(options *sdk.Options)) (*sdk.Integration, error) {
	integration, err := sdk.OpenIntegration(options...)
	if err != nil {
		return nil, err
	}

	integration.Alias = alias
	fmt.Println(fmt.Sprintf("Integration of type: %s, alias: %s successfully defined", integration.GetType(), alias))

	_, err = integration.GetPolicyApplicationPoints(func() string {
		return generateAliasOfSize(3)
	})
	if err != nil {
		return nil, err
	}

	appMap := integration.Apps
	appCount := len(appMap)
	if appCount == 0 {
		fmt.Println("No policy applications discovered.")
		return nil, errors.New("no policy applications discovered")
	}
	fmt.Printf("Successfully loaded %v policy application(s) from %s\n", appCount, alias)
	if appCount > 0 {
		printIntegrationApps(integration)
	}
	return integration, nil
}

func checkFile(path string) error {
	if strings.HasPrefix(path, "~/") {
		usr, _ := user.Current()
		homedir := usr.HomeDir
		path = filepath.Join(homedir, path[2:])
	}
	_, err := os.Stat(path)

	return err
}

func getFile(path string) []byte {
	if strings.HasPrefix(path, "~/") {
		usr, _ := user.Current()
		homedir := usr.HomeDir
		path = filepath.Join(homedir, path[2:])
	}
	fileBytes, _ := os.ReadFile(path)
	return fileBytes
}

type AddGcpIntegrationCmd struct {
	Alias string `arg:"" optional:"" help:"A new local alias that will be used to refer to the integration in subsequent operations. Defaults to an auto-generated alias"`
	File  string `short:"f" xor:"Keyid" required:"" help:"A GCP service account credentials file"`
}

func (a *AddGcpIntegrationCmd) Help() string {
	return `To add a Google GCP integration, specify the location of a Google Credentials file (using --file) whose contents look something like
{
  "type": "service_account",
  "project_id": "google-cloud-project-id",
  "private_key_id": "",
  "private_key": "-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----\n",
  "client_email": "google-cloud-project-id@google-cloud-project-id.iam.gserviceaccount.com",
  "client_id": "000000000000000000000",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/google-cloud-project-id%google-cloud-project-id.iam.gserviceaccount.com"
}

Once a GCP integration is added, it is saved for future use with the supplied alias name.
`
}

func (a *AddGcpIntegrationCmd) Run(cli *CLI) error {
	alias := a.Alias
	if alias == "" {
		alias = generateAliasOfSize(3)

	}

	if cli.Data.GetIntegration(alias) != nil {
		errMsg := fmt.Sprintf("Alias \"%s\" exists", alias)
		if !ConfirmProceed(errMsg + ", overwrite Y[n]") {
			return errors.New(errMsg)
		}
	}
	err := checkFile(a.File)
	if err != nil {
		return err
	}

	keyStr := getFile(a.File)
	if len(keyStr) == 0 {
		return errors.New("invalid or empty Google integration file")
	}

	info := policyprovider.IntegrationInfo{
		Name: sdk.ProviderTypeGoogleCloudIAP,
		Key:  keyStr,
	}

	integration, err := openIntegration(alias, sdk.WithIntegrationInfo(info))
	if err != nil {
		return err
	}
	// printer.Print(appMap)
	cli.Data.Integrations[alias] = integration
	err = cli.Data.Save(&cli.Globals)
	return err
}

type AddOpaIntegrationCmd struct {
	Type         string   `arg:"" required:"" help:"Type of OPA integration: aws, gcp, github, or http"`
	Alias        string   `arg:"" optional:"" help:"A new local alias that will be used to refer to the integration in subsequent operations. Defaults to an auto-generated alias"`
	GitAccount   string   `name:"gitaccount" help:"The account name for a Github project"`
	GitRepo      string   `name:"gitrepo" help:"The repository name for a Github project"`
	GitPath      string   `name:"gitpath" help:"The path to the bundle in a Github project"`
	Bucket       string   `help:"The storage bucket name on AWS or GCP storage service"`
	Object       string   `help:"The storage object name on AWS or GCP storage service"`
	Credential   string   `aliases:"key" help:"A file containing the credential key or token used to access the bundle service or repo"`
	Url          string   `help:"The Http URL for the Bundle service"`
	Cafile       string   `aliases:"cacert" help:"For HTTP integration, public key file (PEM) to verify the server TLS certificate"`
	File         string   `short:"f" help:"File containing a JSON formatted OPA Integration"`
	ClientId     string   `name:"clientid" aliases:"id" help:"OAuth2 Client Id for client credential grant (HTTP Bundle Server)"`
	ClientSecret string   `name:"secret" help:"OAuth2 client secret for obtaining access tokens (HTTP Bundle Server)"`
	TokenUrl     string   `name:"tokenurl" help:"OAuth2 Token Endpoint url for client credential grant flow (HTTP Bundle Server)"`
	Scopes       []string `help:"OAuth2 scopes for client grant flow (HTTP Bundle Server"`
}

func (a *AddOpaIntegrationCmd) Help() string {
	return `To add an OPA integration specify an OPA type ("aws", "gcp", "github" or "http"), and the integration information by
specifying a --file which is a JSON file of the form:

{
  "github": {
    "account": "hexa-org",
    "repo": "opa-bundles",
	"bundlePath": "bundle.tar.gz",
	"key": {
      "accessToken": "some_github_access_token"
    }
  }
}

or

{
  "aws|gcp": {
    "bucket_name": "opa-bundles",
    "object_name": "bundle.tar.gz",
	"key": {
      "region": "us-west-1"
    }
  }
}

or for HTTP:

{ 
  "bundle_url": "https://hexa-bundle-server:8889/bundles/bundle.tar.gz",
  "ca_cert": "pem_encoded_key"
}

or 

{
  "bundle_url": "http://localhost:8889/bundles/bundle.tar.gz",
  "oauth_client": {
    "ClientID": "hexacli",
    "ClientSecret": "abc123",
    "TokenURL": "http://localhost:8080/token",
  }
}

Or, use the parameters in combinations as follows for:
* "gcp", "aws":  specify a --bucket, --object, and --credential
* "github":      specify --account, --repo, --path, and --credential
* "http":        specify --url, and --x509 (optional)

Once the integration is added, it is available for future use with the supplied alias name.
`
}

func (a *AddOpaIntegrationCmd) AfterApply(_ *kong.Context) error {
	if a.File != "" {
		err := checkFile(a.File)
		if err != nil {
			return err
		}
		return nil // if we are using file, all other parameters ignored
	}
	if a.Cafile != "" {
		err := checkFile(a.Cafile)
		if err != nil {
			return errors.New("Unable to open CaFile: " + err.Error())
		}
	}
	if a.Credential != "" {
		err := checkFile(a.Credential)
		if err != nil {
			return errors.New("Unable to locate credential file: " + err.Error())
		}
	}

	switch a.Type {
	case "aws", "gcp":
		if a.Credential == "" {
			return errors.New("missing client credential")
		}
		if a.Bucket == "" {
			return errors.New("missing storage bucket name (--bucket)")
		}
		if a.Object == "" {
			return errors.New("missing storage object name (--object)")
		}

		return nil
	case "github":
		if a.Credential == "" {
			return errors.New("missing github credential")
		}
		if a.GitAccount == "" {
			return errors.New("missing --account parameter")
		}
		if a.GitRepo == "" {
			return errors.New("missing --repo parameter")
		}
		if a.GitPath == "" {
			return errors.New("missing --path parameter for bundle")
		}
		return nil

	case "http":
		if a.Url == "" {
			return errors.New("missing --url for bundle location")
		}
		if a.ClientId != "" || a.ClientSecret != "" || a.TokenUrl != "" {
			if a.ClientId == "" || a.ClientSecret == "" || a.TokenUrl == "" {
				return errors.New("for OAuth2 Client Credentials support, provide all of --clientid, --secret, --tokenurl")
			}
		}
		return nil
	}
	return errors.New("specify a type of 'aws', 'gcp', 'github', or 'http'")
}

func (a *AddOpaIntegrationCmd) Run(cli *CLI) error {
	alias := a.Alias
	if alias == "" {
		alias = generateAliasOfSize(3)

	}

	var integration *sdk.Integration
	var err error
	if a.File != "" {
		keyFileBytes := getFile(a.File)
		info := policyprovider.IntegrationInfo{
			Name: sdk.ProviderTypeOpa,
			Key:  keyFileBytes,
		}

		integration, err = openIntegration(alias, sdk.WithIntegrationInfo(info))

	} else {
		var credBytes []byte
		if a.Credential != "" {
			credBytes = getFile(a.Credential)
		}
		switch a.Type {
		case "aws":
			integration, err = openIntegration(alias, sdk.WithOpaAwsIntegration(a.Bucket, a.Object, credBytes))
		case "gcp":
			integration, err = openIntegration(alias, sdk.WithOpaGcpIntegration(a.Bucket, a.Object, credBytes))
		case "github":
			integration, err = openIntegration(alias, sdk.WithOpaGithubIntegration(a.GitAccount, a.GitRepo, a.GitPath, credBytes))
		case "http":
			var keyFileBytes []byte
			if a.Cafile != "" {
				keyFileBytes = getFile(a.Cafile)
			}

			token := string(credBytes)
			if a.ClientId != "" {
				config := clientcredentials.Config{
					ClientID:     a.ClientId,
					ClientSecret: a.ClientSecret,
					TokenURL:     a.TokenUrl,
					Scopes:       a.Scopes,
				}
				integration, err = openIntegration(alias, sdk.WithOpaHttpOauth2Integration(a.Url, string(keyFileBytes), &config))
			} else {
				integration, err = openIntegration(alias, sdk.WithOpaHttpIntegration(a.Url, string(keyFileBytes), &token))
			}
		}
	}
	if err != nil {
		return err
	}

	cli.Data.Integrations[alias] = integration
	err = cli.Data.Save(&cli.Globals)
	return err
}

type AddAwsIntegrationCmd struct {
	Type   string  `arg:"" required:"" help:"Type of AWS integration: avp, cognito, or apigw"`
	Alias  string  `arg:"" optional:"" help:"A new local alias that will be used to refer to the integration in subsequent operations. Defaults to an auto-generated alias"`
	Region *string `short:"r" help:"The Amazon data center (e.g. us-west-1)"`
	Keyid  *string `short:"k" help:"Amazon Access Key ID"`
	Secret *string `short:"s" help:"Secret access key"`
	File   string  `short:"f" xor:"Keyid" help:"File containing the Amazon credential information"`
}

func (a *AddAwsIntegrationCmd) Help() string {
	return `To add an Amazon integration specify one of "avp", "cognito", or "apigw", and a credential by'
specifying either a file (--file) that contains AWS credentials looks like:
{
  "accessKeyID": "aws-access-key-id",
  "secretAccessKey": "aws-secret-access-key",
  "region": "aws-region"
}

Or, use the parameters --region, --keyid, and --secret to specify the equivalent on the command line.

Once the AWS integration is added, it is available for future use with the supplied alias name.
`
}

func (a *AddAwsIntegrationCmd) AfterApply(_ *kong.Context) error {
	if len(a.File) == 0 {
		if a.Secret == nil || a.Keyid == nil || a.Region == nil {
			return errors.New("must provide all of --keyid, --secret, and --region, or --file")
		}
	} else {
		err := checkFile(a.File)
		if err != nil {
			return err
		}

		if a.Secret != nil || a.Keyid != nil || a.Region != nil {
			return errors.New("must provide all of --keyid, --secret, and --region, or --file")
		}
	}

	switch a.Type {
	case "avp", "cognito", "apigw":
		return nil
	}

	return errors.New("specify the AWS provider type: apigw, avp, or cognito")
}

func (a *AddAwsIntegrationCmd) Run(cli *CLI) error {
	alias := a.Alias
	if alias == "" {
		alias = generateAliasOfSize(3)

	}

	if cli.Data.GetIntegration(alias) != nil {
		errMsg := fmt.Sprintf("Alias \"%s\" exists", alias)
		if !ConfirmProceed(errMsg + ", overwrite Y[n]") {
			return errors.New(errMsg)
		}
	}
	var keyStr []byte
	if len(a.File) != 0 {
		keyStr = getFile(a.File)
	} else {
		keyStr = []byte(fmt.Sprintf(`{
  "accessKeyID": "%s",
  "secretAccessKey": "%s",
  "region": "%s"
}`, *a.Keyid, *a.Secret, *a.Region))
	}

	var provType string // note: validation has already been done
	switch a.Type {
	case "avp":
		provType = sdk.ProviderTypeAvp
	case "apigw":
		provType = sdk.ProviderTypeAwsApiGW
	case "cognito":
		provType = sdk.ProviderTypeCognito
	}

	info := policyprovider.IntegrationInfo{
		Name: provType,
		Key:  keyStr,
	}

	integration, err := openIntegration(alias, sdk.WithIntegrationInfo(info))
	if err != nil {
		return err
	}

	cli.Data.Integrations[alias] = integration
	err = cli.Data.Save(&cli.Globals)
	return err
}

type AddAzureIntegrationCmd struct {
	Alias    string  `arg:"" optional:"" help:"A new local alias that will be used to refer to the integration in subsequent operations. Defaults to an auto-generated alias"`
	Tenant   *string `short:"r" help:"The Azure Tenant id"`
	Clientid *string `short:"c" help:"The Azure Service Principal Client Id (aka appId)"`
	Secret   *string `short:"s" help:"The Azure registration secret"`
	File     string  `short:"f" xor:"Keyid" required:"" help:"File containing the Azure credential information"`
}

func (a *AddAzureIntegrationCmd) Help() string {
	return `To add an Azure integration specify either a file (--file) that integration information that looks like:
{
  "appId": "azure-app-id",
  "secret": "azure-app-registration-secret",
  "tenant": "azure-tenant"
}

Or, use the parameters --tenant, --appid and --secret to specify the equivalent on the command line.

Once the Azure integration is added, it is available for future use with the supplied alias name.
`
}

func (a *AddAzureIntegrationCmd) AfterApply(_ *kong.Context) error {
	if a.File == "" {
		if a.Secret == nil || a.Tenant == nil || a.Clientid == nil {
			return errors.New("must provide all of --tenant, --secret, and --appid, or --file")
		}
	} else {
		err := checkFile(a.File)
		if err != nil {
			return err
		}
		if a.Secret != nil || a.Tenant != nil || a.Clientid != nil {
			return errors.New("must provide all of --tenant, --secret, and --appid, or --file")
		}
	}

	return nil
}

func (a *AddAzureIntegrationCmd) Run(cli *CLI) error {
	alias := a.Alias
	if alias == "" {
		alias = generateAliasOfSize(3)

	}

	if cli.Data.GetIntegration(alias) != nil {
		errMsg := fmt.Sprintf("Alias \"%s\" exists", alias)
		if !ConfirmProceed(errMsg + ", overwrite Y[n]") {
			return errors.New(errMsg)
		}
	}

	var keyStr []byte
	if a.File == "" {
		keyStr = []byte(fmt.Sprintf(`{
  "appId": "%s",
  "secret": "%s",
  "tenant": "%s"
}`, *a.Clientid, *a.Secret, *a.Tenant))
	} else {
		keyStr = getFile(a.File)
	}

	info := policyprovider.IntegrationInfo{
		Name: sdk.ProviderTypeAzure,
		Key:  keyStr,
	}

	integration, err := openIntegration(alias, sdk.WithIntegrationInfo(info))
	if err != nil {
		return err
	}

	cli.Data.Integrations[alias] = integration
	err = cli.Data.Save(&cli.Globals)
	return err
}

type AddCmd struct {
	Aws   AddAwsIntegrationCmd   `cmd:"" aliases:"amazon" help:"Add AWS Api Gateway, Cognito, or AVP integration"`
	Gcp   AddGcpIntegrationCmd   `cmd:"" aliases:"google" help:"Add a Google Cloud GCP integration"`
	Azure AddAzureIntegrationCmd `cmd:"" aliases:"ms,microsoft" help:"Add an Azure RBAC integration"`
	Opa   AddOpaIntegrationCmd   `cmd:"" help:"Add an Open Policy Agent (OPA) integration"`
}

type ExportCmd struct {
	Alias string `arg:"" required:"" help:"Alias for a previously defined integration to export"`
	File  string `arg:"" required:"" help:"Filename to export to (e.g. integration.json)"`
}

func (e *ExportCmd) Run(cli *CLI) error {
	integration := cli.Data.GetIntegration(e.Alias)
	if integration == nil {
		return errors.New(fmt.Sprintf("alias %s not found", e.Alias))
	}
	integrationBytes := integration.Opts.Info.Key

	fmt.Println(fmt.Sprintf("Exporting to: %s ...", e.File))
	err := os.WriteFile(e.File, integrationBytes, 0655)
	return err
}

type GetPolicyApplicationsCmd struct {
	Alias string `arg:"" required:"" help:"Alias for a previously defined integration to retrieve from"`
}

func (a *GetPolicyApplicationsCmd) Run(cli *CLI) error {
	integration := cli.Data.GetIntegration(a.Alias)
	if integration == nil {
		return errors.New(fmt.Sprintf("alias %s not found", a.Alias))
	}

	_, err := integration.GetPolicyApplicationPoints(func() string {
		return generateAliasOfSize(3)
	})
	if err != nil {
		return err
	}

	if len(integration.Apps) == 0 {
		fmt.Println("No policy applications discovered.")
		return nil
	}

	printIntegrationApps(integration)

	return nil
}

type GetPoliciesCmd struct {
	Alias string `arg:"" required:"" help:"Alias or object id of a PAP (application) to retrieve policies from"`
}

func (a *GetPoliciesCmd) Run(cli *CLI) error {
	integration, app := cli.Data.GetApplicationInfo(a.Alias)
	if app == nil {
		return errors.New(fmt.Sprintf("pap alias %s not found", a.Alias))
	}

	policies, err := integration.GetPolicies(a.Alias)
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("Policies retrieved for %s:", a.Alias))

	_ = MarshalJsonNoEscape(policies, os.Stdout)
	outWriter := cli.GetOutputWriter()
	_ = MarshalJsonNoEscape(policies, outWriter.GetOutput())
	outWriter.Close()

	return nil
}

type GetCmd struct {
	Paps     GetPolicyApplicationsCmd `cmd:"" aliases:"apps,applications" help:"Retrieve or discover policy application points from the specified integration alias"`
	Policies GetPoliciesCmd           `cmd:"" aliases:"pol" help:"Get and map policies from a PAP."`
}

type SetPoliciesCmd struct {
	Alias       string `arg:"" required:"" help:"The alias or object id of a PAP (application) where policies are to be set/reconciled with specified policies"`
	File        string `short:"f" required:"" type:"path" help:"A file containing IDQL policy to be applied (REQUIRED)"`
	Differences bool   `optional:"" default:"false" short:"d" help:"When specified, the list of changes to be applied will be shown before confirming change (if supported by provider)"`
}

func (s *SetPoliciesCmd) Run(cli *CLI) error {
	integration, app := cli.Data.GetApplicationInfo(s.Alias)
	if app == nil {
		return errors.New(fmt.Sprintf("pap alias %s not found", s.Alias))
	}

	policies, err := hexapolicysupport.ParsePolicyFile(s.File)
	if err != nil {
		return err
	}

	if s.Differences {
		diffs, err := integration.ReconcilePolicy(s.Alias, policies, false)
		if errors.Is(err, errors.New("provider does not support reconcile")) {
			fmt.Println("Integration provider does not support reconcile.")
		} else {
			for i, diff := range diffs {
				fmt.Println(fmt.Sprintf("%d: %s", i, diff.Report()))
			}
			fmt.Println()
			// Write to output if specified
			output, _ := json.MarshalIndent(diffs, "", "  ")
			cli.GetOutputWriter().WriteBytes(output, true)
		}
	}

	msg := fmt.Sprintf("Applying %d policies to %s", len(policies), s.Alias)
	fmt.Println(msg)
	if ConfirmProceed("Update policies Y|[n]?") {

		res, err := integration.SetPolicyInfo(s.Alias, policies)
		if err != nil {
			return err
		}
		switch res {
		case http.StatusAccepted, http.StatusOK:
			fmt.Println("Policies applied successfully.")
		case http.StatusBadRequest, http.StatusInternalServerError:
			fmt.Println("Bad request or internal processing error")
		case http.StatusUnauthorized, http.StatusForbidden:
			fmt.Println("Request was unauthorized or forbidden")
		default:
			fmt.Println(fmt.Sprintf("Received HTTP Status: %d", res))
		}
	}
	return nil
}

type SetCmd struct {
	Policies SetPoliciesCmd `cmd:"" aliases:"pol,policy" help:"Set policies at a policy application point"`
}

type ShowIntegrationCmd struct {
	Alias string `arg:"" optional:"" help:"An alias for an integration or * to list all. Defaults to listing all"`
}

func printIntegrationInfo(integration *sdk.Integration) {
	title := fmt.Sprintf("Integration: %s", integration.Alias)
	fmt.Println(title)

	fmt.Println(seperatorline[0:len(title)])
	fmt.Println("  Type:   \t" + integration.GetType())
	printIntegrationApps(integration)

}

func (l *ShowIntegrationCmd) Run(cli *CLI) error {
	if l.Alias == "*" || l.Alias == "" {
		if len(cli.Data.Integrations) == 0 {
			fmt.Println("No integrations defined. See 'add' command.")
			return nil
		}
		for _, v := range cli.Data.Integrations {
			printIntegrationInfo(v)
			fmt.Println()
		}
		return nil
	}
	integration := cli.Data.GetIntegration(l.Alias)
	if integration == nil {
		return errors.New(fmt.Sprintf("alias %s not found", l.Alias))
	}
	fmt.Println("Policy Application Points retrieved:")
	fmt.Println()
	printIntegrationApps(integration)
	return nil
}

type ListAppCmd struct {
	Alias string `arg:"" required:"" help:"The alias of an application or integration whose applications are to be listed."`
}

func printApplication(key string, app policyprovider.ApplicationInfo) {
	fmt.Printf("  PAP Alias: %s\n", key)
	fmt.Printf("    ObjectId:   \t%s\n", app.ObjectID)
	fmt.Printf("    Name:       \t%s\n", app.Name)
	fmt.Printf("    Description:\t%s\n", app.Description)
	fmt.Printf("    Service:    \t%s\n", app.Service)
}

func printIntegrationApps(integration *sdk.Integration) {

	for k, app := range integration.Apps {
		fmt.Println()
		printApplication(k, app)
	}
}

func (l *ListAppCmd) Run(cli *CLI) error {
	alias := l.Alias
	integration := cli.Data.GetIntegration(alias)
	if integration != nil {
		fmt.Println("Listing applications for integration " + alias + ":")
		printIntegrationApps(integration)
	} else {
		_, app := cli.Data.GetApplicationInfo(alias)
		if app == nil {
			return errors.New("alias " + alias + " not found")
		}
		printApplication(alias, *app)
	}
	return nil
}

type ShowCmd struct {
	Integration ShowIntegrationCmd `cmd:"" aliases:"int,i" help:"Show locally defined information about a provider integration"`
	Pap         ListAppCmd         `cmd:"" aliases:"app,p,a" help:"Show locally stored information about a policy application"`
	Model       ShowModelCmd       `cmd:"" help:"Show previously loaded Policy Model namespace(s)"`
}

type MapToCmd struct {
	Format string `arg:"" required:"" help:"Target format: gcp, or cedar"`
	File   string `arg:"" type:"path" help:"A file containing IDQL policy to be mapped"`
}

func (m *MapToCmd) AfterApply(_ *kong.Context) error {
	if slices.Contains(MapFormats, strings.ToLower(m.Format)) {
		return nil
	}

	return errors.New(fmt.Sprintf("Invalid format. Valid values are: %v", MapFormats))
}

func (m *MapToCmd) Run(cli *CLI) error {
	fmt.Println(fmt.Sprintf("Mapping IDQL to %s", m.Format))
	policies, err := hexapolicysupport.ParsePolicyFile(m.File)
	if err != nil {
		return err
	}

	switch strings.ToLower(m.Format) {
	case "gcp":
		gcpMapper := gcpBind.New(map[string]string{})
		bindings := gcpMapper.MapPoliciesToBindings(policies)
		_ = MarshalJsonNoEscape(bindings, os.Stdout)
		outWriter := cli.GetOutputWriter()
		_ = MarshalJsonNoEscape(bindings, outWriter.GetOutput())
		outWriter.Close()
	case "cedar":
		cMapper := cedar.NewCedarMapper(map[string]string{})

		cedarPoliciesString, err := cMapper.MapHexaPolicies(m.File, policies)
		if err != nil {
			return err
		}

		fmt.Println(cedarPoliciesString)
		cli.GetOutputWriter().WriteString(cedarPoliciesString, false)
		cli.GetOutputWriter().Close()
	}
	return nil
}

type MapFromCmd struct {
	Format string `arg:"" required:"" help:"Input format: gcp, or cedar"`
	File   string `arg:"" type:"path" help:"A file containing policy to be mapped into IDQL"`
}

func (m *MapFromCmd) AfterApply(_ *kong.Context) error {
	if slices.Contains(MapFormats, strings.ToLower(m.Format)) {
		return nil
	}

	return errors.New(fmt.Sprintf("Invalid format. Valid values are: %v", MapFormats))
}

func (m *MapFromCmd) Run(cli *CLI) error {
	fmt.Println(fmt.Sprintf("Mapping from %s to IDQL", m.Format))
	var policies []hexapolicy.PolicyInfo
	switch strings.ToLower(m.Format) {
	case "gcp":
		gcpMapper := gcpBind.New(map[string]string{})
		assignments, err := gcpBind.ParseFile(m.File)
		if err != nil {
			return err
		}
		policies, err = gcpMapper.MapBindingAssignmentsToPolicy(assignments)
		if err != nil {
			return err
		}

	case "cedar":
		cMapper := cedar.NewCedarMapper(map[string]string{})
		policyBytes, err := os.ReadFile(m.File)
		if err != nil {
			return err
		}
		pols, err := cMapper.MapCedarPolicyBytes(m.File, policyBytes)
		if err != nil {
			return err
		}
		policies = pols.Policies
	}

	_ = MarshalJsonNoEscape(policies, os.Stdout)
	outWriter := cli.GetOutputWriter()
	err := MarshalJsonNoEscape(policies, outWriter.GetOutput())
	if err != nil {
		fmt.Println(err.Error())
	}
	outWriter.WriteString("", true)

	return nil
}

type MapCmd struct {
	To   MapToCmd   `cmd:"" help:"Map IDQL policy to a specified policy format"`
	From MapFromCmd `cmd:"" help:"Map from a specified policy format to IDQL format"`
}

type ReconcileCmd struct {
	AliasSource  string `arg:"" required:"" help:"The alias of a Policy Application, or a file path to a file containing IDQL to act as the source to reconcile against."`
	AliasCompare string `arg:"" required:"" help:"The alias of a Policy Application, or a file path to a file containing IDQL to be reconciled against a source."`
	Differences  bool   `optional:"" short:"d" default:"false" help:"By specifying true, then only the differences are reported (matches are excluded)"`
}

func (r *ReconcileCmd) Run(cli *CLI) error {
	sourceIntegration, appSource := cli.Data.GetApplicationInfo(r.AliasSource)
	compareIntegration, appCompare := cli.Data.GetApplicationInfo(r.AliasCompare)

	var err error
	var comparePolicies []hexapolicy.PolicyInfo
	if appCompare == nil {
		policyBytes, err := os.ReadFile(r.AliasCompare)
		if err != nil {
			return err
		}
		comparePolicies, err = hexapolicysupport.ParsePolicies(policyBytes)
		if err != nil {
			return err
		}
	} else {
		policies, err := compareIntegration.GetPolicies(r.AliasCompare)
		if err != nil {
			return err
		}
		comparePolicies = policies.Policies
	}

	var difs []hexapolicy.PolicyDif
	var sourcePolicies *hexapolicy.Policies

	if appSource == nil {
		// try file path
		policyBytes, err := os.ReadFile(r.AliasSource)
		if err != nil {
			return err
		}
		hexaPolicies, err := hexapolicysupport.ParsePolicies(policyBytes)
		if err != nil {
			return err
		}
		sourcePolicies = &hexapolicy.Policies{Policies: hexaPolicies}
		difs = sourcePolicies.ReconcilePolicies(comparePolicies, r.Differences)
	} else {
		difs, err = sourceIntegration.ReconcilePolicy(r.AliasSource, comparePolicies, r.Differences)
		if err != nil {
			return err
		}

	}

	for i, diff := range difs {
		fmt.Println(fmt.Sprintf("%d: %s", i, diff.Report()))
	}
	fmt.Println()
	// Write to output if specified
	output, _ := json.MarshalIndent(difs, "", "  ")
	cli.GetOutputWriter().WriteBytes(output, true)

	return nil
}

func ConfirmProceed(msg string) bool {
	if msg != "" {
		fmt.Print(msg)
	} else {
		fmt.Print("Proceed Y|[n]? ")
	}

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	if line[0:1] == "Y" {
		return true
	}
	return false
}

type DeleteIntegrationCmd struct {
	Alias string `arg:"" required:"" help:"An alias for an integration to be deleted from local configuration"`
}

func (d DeleteIntegrationCmd) Run(cli *CLI) error {
	integration := cli.Data.GetIntegration(d.Alias)
	if integration == nil {
		return errors.New(fmt.Sprintf("integration %s not found", d.Alias))
	}
	cli.Data.RemoveIntegration(d.Alias)
	return cli.Data.Save(&cli.Globals)
}

type DeletePapCmd struct {
	Alias string `arg:"" required:"" help:"An alias for a policy application (PAP) to be deleted from local configuration"`
}

func (d DeletePapCmd) Run(cli *CLI) error {
	_, app := cli.Data.GetApplicationInfo(d.Alias)
	if app == nil {
		return errors.New(fmt.Sprintf("policy application %s not found", d.Alias))
	}
	cli.Data.RemoveApplication(d.Alias)
	return cli.Data.Save(&cli.Globals)
}

type DeleteCmd struct {
	Integration DeleteIntegrationCmd `cmd:"" aliases:"i,int" help:"Delete an integration from local configuration"`
	Pap         DeletePapCmd         `cmd:"" aliases:"app,PAP" help:"Delete an application from local configuration"`
}

type ExitCmd struct {
}

func (e *ExitCmd) Run(globals *Globals) error {
	err := globals.Data.Save(globals)
	if err != nil {
		fmt.Println(err.Error())
		if ConfirmProceed("Abort exit? Y|[n] ") {
			return nil
		}
	}
	os.Exit(-1)
	return nil
}

type HelpCmd struct {
	Command []string `arg:"" optional:"" help:"Show help on command."`
}

// Run shows help.
func (h *HelpCmd) Run(realCtx *kong.Context) error {
	ctx, err := kong.Trace(realCtx.Kong, h.Command)
	if err != nil {
		return err
	}
	if ctx.Error != nil {
		return ctx.Error
	}
	// fmt.Printf("Args:\t%v\n", ctx.Args)
	// fmt.Printf("Command:\t%s\n", ctx.Command())
	err = ctx.PrintUsage(false)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(realCtx.Stdout)
	return nil
}

func MarshalJsonNoEscape(t interface{}, out io.Writer) error {
	if out == nil {
		return nil // do nothing
	}
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(t)
	return err
}

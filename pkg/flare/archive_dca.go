// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package flare

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	flarehelpers "github.com/DataDog/datadog-agent/comp/core/flare/helpers"
	apiv1 "github.com/DataDog/datadog-agent/pkg/clusteragent/api/v1"
	"github.com/DataDog/datadog-agent/pkg/clusteragent/custommetrics"
	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/status"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes/apiserver"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

const (
	HELM_CHART_RELEASE_NAME       = "DD_CHART_RELEASE_NAME"
	HELM_CHART_RELEASE_NAMESPACE  = "DD_KUBE_RESOURCES_NAMESPACE"
	HELM_AGENT_DAEMONSET          = "DD_AGENT_DAEMONSET"
	HELM_CLUSTER_AGENT_DEPLOYMENT = "DD_CLUSTER_AGENT_DEPLOYMENT"
)

// CreateDCAArchive packages up the files
func CreateDCAArchive(local bool, distPath, logFilePath string) (string, error) {
	fb, err := flarehelpers.NewFlareBuilder()
	if err != nil {
		return "", err
	}

	confSearchPaths := SearchPaths{
		"":     config.Datadog.GetString("confd_path"),
		"dist": filepath.Join(distPath, "conf.d"),
	}

	createDCAArchive(fb, local, confSearchPaths, logFilePath)
	return fb.Save()
}

func createDCAArchive(fb flarehelpers.FlareBuilder, local bool, confSearchPaths SearchPaths, logFilePath string) {
	// If the request against the API does not go through we don't collect the status log.
	if local {
		fb.AddFile("local", []byte(""))
	} else {
		// The Status will be unavailable unless the agent is running.
		// Only zip it up if the agent is running
		err := fb.AddFileFromFunc("cluster-agent-status.log", status.GetAndFormatDCAStatus)
		if err != nil {
			log.Errorf("Error getting the status of the DCA, %q", err)
			return
		}
	}

	getLogFiles(fb, logFilePath)
	getConfigFiles(fb, confSearchPaths)
	getClusterAgentConfigCheck(fb)   //nolint:errcheck
	getExpVar(fb)                    //nolint:errcheck
	getMetadataMap(fb)               //nolint:errcheck
	getClusterAgentClusterChecks(fb) //nolint:errcheck
	getClusterAgentDiagnose(fb)      //nolint:errcheck
	getAgentDaemonSet(fb)            //nolint:errcheck
	getClusterAgentDeployment(fb)    //nolint:errcheck
	getHelmValues(fb)                //nolint:errcheck
	fb.AddFileFromFunc("envvars.log", getEnvVars)
	fb.AddFileFromFunc("telemetry.log", QueryDCAMetrics)
	fb.AddFileFromFunc("tagger-list.json", getDCATaggerList)
	fb.AddFileFromFunc("workload-list.log", getDCAWorkloadList)

	if config.Datadog.GetBool("external_metrics_provider.enabled") {
		getHPAStatus(fb) //nolint:errcheck
	}
}

// QueryDCAMetrics gets the metrics payload exposed by the cluster agent
func QueryDCAMetrics() ([]byte, error) {
	r, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", config.Datadog.GetInt("metrics_port")))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	return ioutil.ReadAll(r.Body)
}

func getMetadataMap(fb flarehelpers.FlareBuilder) error {
	metaList := apiv1.NewMetadataResponse()
	cl, err := apiserver.GetAPIClient()
	if err != nil {
		metaList.Errors = fmt.Sprintf("Can't create client to query the API Server: %s", err.Error())
	} else {
		// Grab the metadata map for all nodes.
		metaList, err = apiserver.GetMetadataMapBundleOnAllNodes(cl)
		if err != nil {
			log.Infof("Error while collecting the cluster level metadata: %q", err)
		}
	}

	metaBytes, err := json.Marshal(metaList)
	if err != nil {
		return log.Errorf("Error while marshalling the cluster level metadata: %q", err)
	}

	str, err := status.FormatMetadataMapCLI(metaBytes)
	if err != nil {
		return log.Errorf("Error while rendering the cluster level metadata: %q", err)
	}

	return fb.AddFile("cluster-agent-metadatamapper.log", []byte(str))
}

func getClusterAgentClusterChecks(fb flarehelpers.FlareBuilder) error {
	var b bytes.Buffer

	writer := bufio.NewWriter(&b)
	GetClusterChecks(writer, "") //nolint:errcheck
	writer.Flush()

	return fb.AddFile("clusterchecks.log", b.Bytes())
}

func getHPAStatus(fb flarehelpers.FlareBuilder) error {
	stats := make(map[string]interface{})
	apiCl, err := apiserver.GetAPIClient()
	if err != nil {
		stats["custommetrics"] = map[string]string{"Error": err.Error()}
	} else {
		stats["custommetrics"] = custommetrics.GetStatus(apiCl.Cl)
	}
	statsBytes, err := json.Marshal(stats)
	if err != nil {
		return log.Errorf("Error while marshalling the cluster level metadata: %q", err)
	}

	str, err := status.FormatHPAStatus(statsBytes)
	if err != nil {
		return log.Errorf("Could not collect custommetricsprovider.log: %s", err)
	}

	return fb.AddFile("custommetricsprovider.log", []byte(str))
}

func getClusterAgentConfigCheck(fb flarehelpers.FlareBuilder) error {
	var b bytes.Buffer

	writer := bufio.NewWriter(&b)
	GetClusterAgentConfigCheck(writer, true) //nolint:errcheck
	writer.Flush()

	return fb.AddFile("config-check.log", b.Bytes())
}

func getClusterAgentDiagnose(fb flarehelpers.FlareBuilder) error {
	var b bytes.Buffer

	writer := bufio.NewWriter(&b)
	GetClusterAgentDiagnose(writer) //nolint:errcheck
	writer.Flush()

	return fb.AddFile("diagnose.log", b.Bytes())
}

func getDCATaggerList() ([]byte, error) {
	ipcAddress, err := config.GetIPCAddress()
	if err != nil {
		return nil, err
	}

	taggerListURL := fmt.Sprintf("https://%v:%v/tagger-list", ipcAddress, config.Datadog.GetInt("cluster_agent.cmd_port"))

	return getTaggerList(taggerListURL)
}

func getDCAWorkloadList() ([]byte, error) {
	ipcAddress, err := config.GetIPCAddress()
	if err != nil {
		return nil, err
	}

	return getWorkloadList(fmt.Sprintf("https://%v:%v/workload-list?verbose=true", ipcAddress, config.Datadog.GetInt("cluster_agent.cmd_port")))
}

// getAgentDaemonSet retrieves the DaemonSet manifest of the Agent
func getAgentDaemonSet(fb flarehelpers.FlareBuilder) error {
	// The Agent DaemonSet name is based on the Helm chart template and added to the Cluster Agent as an environment variable
	var agentDaemonsetName string
	var releaseNamespace string
	var agentDaemonset []byte

	cl, err := apiserver.GetAPIClient()
	if err != nil {
		log.Errorf("Can't create client to query the API Server: %s", err)
	} else {
		agentDaemonsetName = os.Getenv(HELM_AGENT_DAEMONSET)
		releaseNamespace = os.Getenv(HELM_CHART_RELEASE_NAMESPACE)
		agentDaemonset, err = apiserver.GetDs(cl, agentDaemonsetName, releaseNamespace)
		if err != nil {
			log.Debugf("Error while collecting the Agent DaemonSet: %q", err)
		}
	}

	return fb.AddFile("agent-daemonset.yaml", agentDaemonset)
}

// getClusterAgentDeployment retrieves the Deployment manifest of the Cluster Agent
func getClusterAgentDeployment(fb flarehelpers.FlareBuilder) error {
	// The Cluster Agent Deployment name is based on the Helm chart template and added to the Cluster Agent as an environment variable
	var clusterAgentDeploymentName string
	var releaseNamespace string
	var clusterAgentDeployment []byte

	cl, err := apiserver.GetAPIClient()
	if err != nil {
		log.Errorf("Can't create client to query the API Server: %s", err)
	} else {
		clusterAgentDeploymentName = os.Getenv(HELM_CLUSTER_AGENT_DEPLOYMENT)
		releaseNamespace = os.Getenv(HELM_CHART_RELEASE_NAMESPACE)
		clusterAgentDeployment, err = apiserver.GetDeploy(cl, clusterAgentDeploymentName, releaseNamespace)
		if err != nil {
			log.Debugf("Error while collecting the Cluster Agent Deployment: %q", err)
		}
	}

	return fb.AddFile("cluster-agent-deployment.yaml", clusterAgentDeployment)
}

// getHelmValues retrieves the user-defined values for the Datadog Helm chart
func getHelmValues(fb flarehelpers.FlareBuilder) error {
	var helmUserValues []byte
	var releaseName string
	var releaseNamespace string

	cl, err := apiserver.GetAPIClient()
	if err != nil {
		log.Errorf("Can't create client to query the API Server: %s", err)
	} else {
		releaseName = os.Getenv(HELM_CHART_RELEASE_NAME)
		releaseNamespace = os.Getenv(HELM_CHART_RELEASE_NAMESPACE)
		helmUserValues, err = apiserver.GetDatadogHelmUserValuesFromSecret(cl, releaseName, releaseNamespace)
		if err != nil {
			log.Warnf("Error while collecting the Helm chart values from secret: %v", err)
		}
		// The returned bytes array is not nil, we can exit
		if len(helmUserValues) != 0 {
			return fb.AddFile("helm-values.yaml", helmUserValues)
		}
		// The cluster Agent was unable to retrieve Helm chart data from secrets, attempting to retrieve them from Configmaps
		helmUserValues, err = apiserver.GetDatadogHelmUserValuesFromConfigmap(cl, releaseName, releaseNamespace)
		if err != nil {
			log.Warnf("Error while collecting the Helm chart values from configmap: %v", err)
		}
		// The returned bytes array is not nil, we can exit
		if len(helmUserValues) != 0 {
			return fb.AddFile("helm-values.yaml", helmUserValues)
		}
	}
	return nil
}

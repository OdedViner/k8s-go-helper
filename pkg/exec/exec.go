package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"k8s.io/client-go/tools/clientcmd"
	rookclient "github.com/rook/rook/pkg/client/clientset/versioned"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"	
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Clientsets struct {
	// The Kubernetes config used for these client sets
	KubeConfig *rest.Config

	// Kube is a connection to the core Kubernetes API
	Kube kubernetes.Interface

	// Rook is a typed connection to the rook API
	Rook rookclient.Interface

	// Dynamic is used for manage dynamic resources
	Dynamic dynamic.Interface
}

func getClientsets(ctx context.Context) *Clientsets {
	var err error
	var kubeContext string
	clientsets := &Clientsets{}

	congfigOverride := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		congfigOverride = &clientcmd.ConfigOverrides{CurrentContext: kubeContext}
	}

	// 1. Create Kubernetes Client
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		congfigOverride,
	)

	clientsets.KubeConfig, err = kubeconfig.ClientConfig()
	if err != nil {
		fmt.Println(err)
	}

	clientsets.Rook, err = rookclient.NewForConfig(clientsets.KubeConfig)
	if err != nil {
		fmt.Println(err)
	}

	clientsets.Kube, err = k8s.NewForConfig(clientsets.KubeConfig)
	if err != nil {
		fmt.Println(err)
	}

	clientsets.Dynamic, err = dynamic.NewForConfig(clientsets.KubeConfig)
	if err != nil {
		fmt.Println(err)
	}

	return clientsets
}

// execCmdInToolPodDebug exec command on specific pod and wait the command's output.
func execCmdInToolPodDebug(commandStr string)  (string, string, error)  {
	ctx := context.TODO()
	clientsets := getClientsets(ctx)
	podNamespace := "rook-ceph"
	clusterNamespace := "rook-ceph"
    var stdout, stderr io.Writer = &bytes.Buffer{}, &bytes.Buffer{}
	returnOutput := true
	pods, err := clientsets.Kube.CoreV1().Pods(clusterNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=rook-ceph-tools",
	})
	if err != nil {
		return "a", "a", fmt.Errorf("failed to get ceph tool pod. %w", err)
	}
	cmd := strings.Fields(commandStr)
	podName := pods.Items[0].ObjectMeta.Name
	containerName := "rook-ceph-tools"

	// Prepare the API URL used to execute another process within the Pod.  In
	// this case, we'll run a remote shell.
	req := clientsets.Kube.CoreV1().RESTClient().
		Post().
		Namespace(podNamespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientsets.KubeConfig, "POST", req.URL())
	if err != nil {
		return "a", "a", fmt.Errorf("failed to create SPDYExecutor. %w", err)
	}

	// returnOutput is true, the command's output will be print on shell directly with os.Stdout or os.Stderr
	if !returnOutput {
		// Connect this process' std{in,out,err} to the remote shell process.
		err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Tty:    false,
		})
	} else {
		// Connect this process' std{in,out,err} to the remote shell process.
		err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: stdout,
			Stderr: stderr,
			Tty:    false,
		})
	}
	if err != nil {
		return "a", "a", fmt.Errorf("failed to run command. %w", err)
	}
	outputSting := stdout.(*bytes.Buffer)
	return outputSting.String(), "" , nil
}

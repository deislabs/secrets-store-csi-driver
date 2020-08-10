package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"k8s.io/klog"
)

// copied from https://github.com/kubernetes-sigs/cluster-api/blob/v0.3.6/test/framework/exec/kubectl.go#L26
// TODO: Remove this usage of kubectl and replace with a function from apply.go using the controller-runtime client.
func KubectlApply(ctx context.Context, kubeconfigPath string, resources []byte) error {
	rbytes := bytes.NewReader(resources)
	applyCmd := NewCommand(
		WithCommand("kubectl"),
		WithArgs("apply", "--kubeconfig", kubeconfigPath, "-f", "-"),
		WithStdin(rbytes),
	)
	stdout, stderr, err := applyCmd.Run(ctx)
	if err != nil {
		fmt.Println(string(stderr))
		return err
	}
	fmt.Println(string(stdout))
	return nil
}

func execLocal(input io.Reader, cmd string, args ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	klog.Infof("%s %s", cmd, strings.Join(args, " "))
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if input != nil {
		command.Stdin = input
	}
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func execWithInput(input []byte, cmd string, args ...string) (stdout []byte, stderr []byte, e error) {
	var r io.Reader
	if input != nil {
		r = bytes.NewReader(input)
	}
	return execLocal(r, cmd, args...)
}

func Kubectl(args ...string) ([]byte, []byte, error) {
	return execLocal(nil, "kubectl", args...)
}

func KubectlWithInput(input []byte, args ...string) ([]byte, []byte, error) {
	return execWithInput(input, "kubectl", args...)
}

func KubectlExec(kubeconfigPath, podName, namespace string, args ...string) ([]byte, []byte, error) {
	args = append([]string{
		"exec",
		fmt.Sprintf("--kubeconfig=%s", kubeconfigPath),
		fmt.Sprintf("--namespace=%s", namespace),
		podName,
		"--",
	}, args...)

	return Kubectl(args...)
}

func KubectlExecWithInput(input []byte, kubeconfigPath, podName, namespace string, args ...string) ([]byte, []byte, error) {
	args = append([]string{
		"exec",
		"-it",
		fmt.Sprintf("--kubeconfig=%s", kubeconfigPath),
		fmt.Sprintf("--namespace=%s", namespace),
		podName,
		"--",
	}, args...)

	return KubectlWithInput(input, args...)
}

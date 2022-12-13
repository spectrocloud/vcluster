package lifecycle

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/loft-sh/utils/pkg/log"
	"github.com/loft-sh/vcluster/pkg/constants"
)

// Pause

func PauseVCluster(kubeClient *kubernetes.Clientset, name, namespace string, log log.Logger) error {
	// scale down vcluster itself
	labelSelector := "app=vcluster,release=" + name
	found, err := scaleDownStatefulSet(kubeClient, labelSelector, namespace, log)
	if err != nil {
		return err
	} else if !found {
		found, err = scaleDownDeployment(kubeClient, labelSelector, namespace, log)
		if err != nil {
			return err
		} else if !found {
			return errors.Errorf("couldn't find vcluster %s in namespace %s", name, namespace)
		}

		// scale down kube api server
		_, err = scaleDownDeployment(kubeClient, "app=vcluster-api,release="+name, namespace, log)
		if err != nil {
			return err
		}

		// scale down kube controller
		_, err = scaleDownDeployment(kubeClient, "app=vcluster-controller,release="+name, namespace, log)
		if err != nil {
			return err
		}

		// scale down etcd
		_, err = scaleDownStatefulSet(kubeClient, "app=vcluster-etcd,release="+name, namespace, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteVClusterWorkloads(kubeClient *kubernetes.Clientset, labelSelector, namespace string, log log.Logger) error {
	list, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}

	if len(list.Items) > 0 {
		log.Infof("Delete %d vcluster workloads", len(list.Items))
		for _, item := range list.Items {
			err = kubeClient.CoreV1().Pods(namespace).Delete(context.TODO(), item.Name, metav1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "delete pod %s/%s", namespace, item.Name)
			}
		}
	}

	return nil
}

func scaleDownDeployment(kubeClient kubernetes.Interface, labelSelector, namespace string, log log.Logger) (bool, error) {
	list, err := kubeClient.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return false, err
	} else if len(list.Items) == 0 {
		return false, nil
	}

	zero := int32(0)
	for _, item := range list.Items {
		if item.Annotations != nil && item.Annotations[constants.PausedAnnotation] == "true" {
			log.Infof("vcluster %s/%s is already paused", namespace, item.Name)
			return true, nil
		} else if item.Spec.Replicas != nil && *item.Spec.Replicas == 0 {
			continue
		}

		originalObject := item.DeepCopy()
		if item.Annotations == nil {
			item.Annotations = map[string]string{}
		}

		replicas := 1
		if item.Spec.Replicas != nil {
			replicas = int(*item.Spec.Replicas)
		}

		item.Annotations[constants.PausedAnnotation] = "true"
		item.Annotations[constants.PausedReplicasAnnotation] = strconv.Itoa(replicas)
		item.Spec.Replicas = &zero

		patch := client.MergeFrom(originalObject)
		data, err := patch.Data(&item)
		if err != nil {
			return false, errors.Wrap(err, "create deployment patch")
		}

		// patch deployment
		log.Infof("Scale down deployment %s/%s...", namespace, item.Name)
		_, err = kubeClient.AppsV1().Deployments(namespace).Patch(context.TODO(), item.Name, patch.Type(), data, metav1.PatchOptions{})
		if err != nil {
			return false, errors.Wrap(err, "patch deployment")
		}

		// wait until deployment is scaled down
		err = wait.PollImmediate(time.Second, time.Minute*3, func() (done bool, err error) {
			deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), item.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			return deployment.Status.Replicas == 0, nil
		})
		if err != nil {
			return false, errors.Wrap(err, "wait for deployment scaled down")
		}
	}

	return true, nil
}

func scaleDownStatefulSet(kubeClient kubernetes.Interface, labelSelector, namespace string, log log.Logger) (bool, error) {
	list, err := kubeClient.AppsV1().StatefulSets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return false, err
	} else if len(list.Items) == 0 {
		return false, nil
	}

	zero := int32(0)
	for _, item := range list.Items {
		if item.Annotations != nil && item.Annotations[constants.PausedAnnotation] == "true" {
			log.Infof("vcluster %s/%s is already paused", namespace, item.Name)
			return true, nil
		} else if item.Spec.Replicas != nil && *item.Spec.Replicas == 0 {
			continue
		}

		originalObject := item.DeepCopy()
		if item.Annotations == nil {
			item.Annotations = map[string]string{}
		}

		replicas := 1
		if item.Spec.Replicas != nil {
			replicas = int(*item.Spec.Replicas)
		}

		item.Annotations[constants.PausedAnnotation] = "true"
		item.Annotations[constants.PausedReplicasAnnotation] = strconv.Itoa(replicas)
		item.Spec.Replicas = &zero

		patch := client.MergeFrom(originalObject)
		data, err := patch.Data(&item)
		if err != nil {
			return false, errors.Wrap(err, "create statefulSet patch")
		}

		// patch deployment
		log.Infof("Scale down statefulSet %s/%s...", namespace, item.Name)
		_, err = kubeClient.AppsV1().StatefulSets(namespace).Patch(context.TODO(), item.Name, patch.Type(), data, metav1.PatchOptions{})
		if err != nil {
			return false, errors.Wrap(err, "patch statefulSet")
		}

		// wait until deployment is scaled down
		err = wait.PollImmediate(time.Second, time.Minute*3, func() (done bool, err error) {
			obj, err := kubeClient.AppsV1().StatefulSets(namespace).Get(context.TODO(), item.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			return obj.Status.Replicas == 0, nil
		})
		if err != nil {
			return false, errors.Wrap(err, "wait for statefulSet scaled down")
		}
	}

	return true, nil
}

// Resume

func ResumeVCluster(kubeClient *kubernetes.Clientset, name, namespace string, log log.Logger) error {
	// scale down vcluster itself
	labelSelector := "app=vcluster,release=" + name
	found, err := scaleUpStatefulSet(kubeClient, labelSelector, namespace, log)
	if err != nil {
		return err
	} else if !found {
		found, err = scaleUpDeployment(kubeClient, labelSelector, namespace, log)
		if err != nil {
			return err
		} else if !found {
			return errors.Errorf("couldn't find a paused vcluster %s in namespace %s. Make sure the vcluster exists and was paused previously", name, namespace)
		}

		// scale down kube api server
		_, err = scaleUpDeployment(kubeClient, "app=vcluster-api,release="+name, namespace, log)
		if err != nil {
			return err
		}

		// scale down kube controller
		_, err = scaleUpDeployment(kubeClient, "app=vcluster-controller,release="+name, namespace, log)
		if err != nil {
			return err
		}

		// scale down etcd
		_, err = scaleUpStatefulSet(kubeClient, "app=vcluster-etcd,release="+name, namespace, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func scaleUpDeployment(kubeClient kubernetes.Interface, labelSelector string, namespace string, log log.Logger) (bool, error) {
	list, err := kubeClient.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return false, err
	} else if len(list.Items) == 0 {
		return false, nil
	}

	for _, item := range list.Items {
		if item.Annotations == nil || item.Annotations[constants.PausedAnnotation] != "true" {
			return false, nil
		}

		originalObject := item.DeepCopy()

		replicas := 1
		if item.Annotations[constants.PausedReplicasAnnotation] != "" {
			replicas, err = strconv.Atoi(item.Annotations[constants.PausedReplicasAnnotation])
			if err != nil {
				log.Errorf("error parsing old replicas: %v", err)
				replicas = 1
			}
		}

		replicas32 := int32(replicas)
		delete(item.Annotations, constants.PausedAnnotation)
		delete(item.Annotations, constants.PausedReplicasAnnotation)
		item.Spec.Replicas = &replicas32

		patch := client.MergeFrom(originalObject)
		data, err := patch.Data(&item)
		if err != nil {
			return false, errors.Wrap(err, "create deployment patch")
		}

		// patch deployment
		_, err = kubeClient.AppsV1().Deployments(namespace).Patch(context.TODO(), item.Name, patch.Type(), data, metav1.PatchOptions{})
		if err != nil {
			return false, errors.Wrap(err, "patch deployment")
		}
	}

	return true, nil
}

func scaleUpStatefulSet(kubeClient kubernetes.Interface, labelSelector string, namespace string, log log.Logger) (bool, error) {
	list, err := kubeClient.AppsV1().StatefulSets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return false, err
	} else if len(list.Items) == 0 {
		return false, nil
	}

	for _, item := range list.Items {
		if item.Annotations == nil || item.Annotations[constants.PausedAnnotation] != "true" {
			return false, nil
		}

		originalObject := item.DeepCopy()

		replicas := 1
		if item.Annotations[constants.PausedReplicasAnnotation] != "" {
			replicas, err = strconv.Atoi(item.Annotations[constants.PausedReplicasAnnotation])
			if err != nil {
				log.Errorf("error parsing old replicas: %v", err)
				replicas = 1
			}
		}

		replicas32 := int32(replicas)
		delete(item.Annotations, constants.PausedAnnotation)
		delete(item.Annotations, constants.PausedReplicasAnnotation)
		item.Spec.Replicas = &replicas32

		patch := client.MergeFrom(originalObject)
		data, err := patch.Data(&item)
		if err != nil {
			return false, errors.Wrap(err, "create statefulSet patch")
		}

		// patch deployment
		_, err = kubeClient.AppsV1().StatefulSets(namespace).Patch(context.TODO(), item.Name, patch.Type(), data, metav1.PatchOptions{})
		if err != nil {
			return false, errors.Wrap(err, "patch statefulSet")
		}
	}

	return true, nil
}

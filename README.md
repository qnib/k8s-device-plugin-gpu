# k8s-device-plugin-gpu
K8s device plugin to allow GPU device scheduling using HOUDINI.

## Description
This plugin uses [HOUDINI](https://github.com/qnib/moby/tree/houdini) to allow kubernetes objects to request GPU resources.

## Show Me

```bash
$ kubectl apply -f k8s/qnib-device-plugin.yml
daemonset.extensions "qnib-device-plugin-gpu-daemonset" created
$ kubectl -n kube-system get daemonset |grep qnib
qnib-device-plugin-gpu-daemonset   1         1         1         1            1           houdini.gpu=true                4m
```

Now pods can request the resource `qnib.org/gpu`. Let's spin up a bunch of batch jobs.

```
$ for x in {1..5};do sed -e "s/\-0/\-${x}/" k8s/job-nvidia-smi.yml|kubectl apply -f -;done
job.batch "nvidia-smi-1" created
job.batch "nvidia-smi-2" created
job.batch "nvidia-smi-3" created
job.batch "nvidia-smi-4" created
job.batch "nvidia-smi-5" created
$ kubectl get all
NAME                     READY     STATUS              RESTARTS   AGE
pod/nvidia-smi-1-st6vv   0/1       Completed           0          5s
pod/nvidia-smi-2-jpg76   0/1       Pending             0          4s
pod/nvidia-smi-3-swfg5   0/1       ContainerCreating   0          4s
pod/nvidia-smi-4-wzs5c   0/1       Pending             0          3s
pod/nvidia-smi-5-256n4   0/1       Pending             0          3s

NAME                     DESIRED   SUCCESSFUL   AGE
job.batch/nvidia-smi-1   1         1            5s
job.batch/nvidia-smi-2   1         0            4s
job.batch/nvidia-smi-3   1         0            4s
job.batch/nvidia-smi-4   1         0            3s
```

This jobs simply runs `nvidia-smi -L` to list all available GPUs in the pod.

```
$ kubectl logs pod/nvidia-smi-1-st6vv
GPU 0: GRID K520 (UUID: GPU-897b9b83-1c53-8b0f-b5de-f537f8f80db3)
```

The jobs will be in status `Pending`, since they have to wait until the resource is released by the previous job.
```
$ kubectl describe pod/nvidia-smi-2-jpg76 |grep -A10 ^Events
Events:
  Type     Reason            Age              From                                             Message
  ----     ------            ----             ----                                             -------
  Warning  FailedScheduling  3m (x8 over 3m)  default-scheduler                                0/2 nodes are available: 2 Insufficient qnib.org/gpu.
  Normal   Scheduled         3m               default-scheduler                                Successfully assigned default/nvidia-smi-2-jpg76 to christiankniep-testkit-b26f94-ubuntu-0
  Normal   Pulling           3m               kubelet, christiankniep-testkit-b26f94-ubuntu-0  pulling image "ubuntu"
  Normal   Pulled            3m               kubelet, christiankniep-testkit-b26f94-ubuntu-0  Successfully pulled image "ubuntu"
  Normal   Created           3m               kubelet, christiankniep-testkit-b26f94-ubuntu-0  Created container
  Normal   Started           3m               kubelet, christiankniep-testkit-b26f94-ubuntu-0  Started container
```

Once everyone had the chance, they are finished.

```
$ kubectl get all
NAME                     READY     STATUS      RESTARTS   AGE
pod/nvidia-smi-1-st6vv   0/1       Completed   0          25s
pod/nvidia-smi-2-jpg76   0/1       Completed   0          24s
pod/nvidia-smi-3-swfg5   0/1       Completed   0          24s
pod/nvidia-smi-4-wzs5c   0/1       Completed   0          23s
pod/nvidia-smi-5-256n4   0/1       Completed   0          23s

NAME                     DESIRED   SUCCESSFUL   AGE
job.batch/nvidia-smi-1   1         1            25s
job.batch/nvidia-smi-2   1         1            24s
job.batch/nvidia-smi-3   1         1            24s
job.batch/nvidia-smi-4   1         1            23s
job.batch/nvidia-smi-5   1         1            23s
```


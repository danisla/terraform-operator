define KANIKO_DEPLOY
apiVersion: v1
kind: Pod
metadata:
  name: kaniko
spec:
  containers:
  - name: kaniko
    image: gcr.io/kaniko-project/executor:latest
    args: ["--dockerfile=<path to Dockerfile>",
            "--context=gs://<GCS bucket>/<path to .tar.gz>",
            "--destination=<gcr.io/$PROJECT/$IMAGE:$TAG>"]
    volumeMounts:
      - name: kaniko-secret
        mountPath: /secret
    env:
      - name: GOOGLE_APPLICATION_CREDENTIALS
        value: /secret/kaniko-secret.json
  restartPolicy: Never
  volumes:
    - name: kaniko-secret
      secret:
        secretName: {{SECRET_NAME}}
endef
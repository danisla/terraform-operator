KANIKO_SA_KEY := $(HOME)/.kaniko-sa-key.json

kaniko-secret: $(KANIKO_SA_KEY)
	kubectl create secret generic kaniko-secret --from-file=kaniko-secret=$(KANIKO_SA_KEY)
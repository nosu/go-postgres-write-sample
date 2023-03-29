### Create resources

```
gcloud auth application-default login
cd terraform
terraform apply -auto-approve
cd ..
```

### Deploy Cloud Run

```
gcloud config set core/project alloydb-test-381909
gcloud run deploy db-write-webapi --source . --region us-central1
```


```
docker-compose up
```

```
export PORJECT_ID=alloydb-test-381909;export DB_USER=user;export DB_PASS=password;export DB_HOST=test;export DB_PORT=5432;export DB_NAME=test;ENVIRONMENT=LOCAL
go run main.go
```
sudo mv /var/lib/dpkg/info/containerd.io.* /tmp/ && sudo mv /var/lib/dpkg/info/docker-ce-cli.* /tmp/ && sudo mv /var/lib/dpkg/info/docker-ce.* /tmp/
sudo dpkg --remove --force-remove-reinstreq containerd.io docker-ce-cli docker-ce
sudo apt-get remove containerd.io docker-ce-cli docker-ce
sudo apt autoremove && sudo apt autoclean
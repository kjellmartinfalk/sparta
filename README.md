# sparta

## examples

having two different configurations, one called `version_1.yaml`
```yaml
ssm_params:
  database_config: "/my_app/version_1/database-endpoint" # single value
  auth_config: "/my_app/version_1/auth-config" # json formatted value
```

and another called `version_2.yaml`

```yaml
ssm_params:
  database_config: "/my_app/version_2/database-endpoint" # single value
  auth_config: "/my_app/version_2/auth-config" # json formatted value
```

with a template called `template.yaml` in `/template` 
```yaml
my_app:
  database_url: {{ .database_config "endpoint" }}
  database_user: {{ jsonField .auth_config "credentials.username" }}
  database_password: {{ jsonField .auth_config "credentials.password" }}
```

you can run the following

```bash
$ sparta --template template/template.yaml --config version-1.yaml --config version-2.yaml 
```

which "would" output the following to stdout

```yaml
my_app:
  database_url: tcp://192.168.0.1
  database_user: user
  database_password: ******
```

additionally you can run 
```bash
$ sparta --template template.yaml --config config.yaml --output ./output # will be written to /output/config.yaml
$ sparta --template templates/ --config config.yaml
```
service "dispatcher" { 
  buildpack "go" { version = "1.13.1" }
  depends_on = [ "service.api" ]

  setup { 
  }

  build {
    commands = [
      "go get",
      "go build"
    ]
  }

  test {
    command = "go test ./.."
  }

  run { 
    environment {
      variable "YB_API_URL" { 
        value = "http://${services.api.ip}:${services.api.ports.http}"
      }
    }
    command =  "./dispatcher"
  }
}

service "api" {
  buildpack "python" { version = "3.7.3" }

  variable "db_user" { value = "yourbase" }
  variable "db_pass" { value = "yourbase" }
  variable "db_name" { value = "yourbase" }

  port "http" { 
    type = "tcp"
    port = "5001"
  }
  port "https" { 
    type = "tcp" 
    port = "5000"
  }

  container "db" { 
    image = "postgres:9.3"
    mounts = [ "pg_data:/var/lib/postgresql/data" ]
    environment {
        variable "POSTGRES_PASSWORD" { value = "${services.api.vars.db_pass}" }
        variable "POSTGRES_USER" { value = "${services.api.vars.db_user}" }
        variable "POSTGRES_DB"  { value = "${services.api.vars.db_name}" }
    }
  }
    
  setup {
    command = "pip install requirements.txt"
  }
 
  build { }

  test {
     command = "env"
  }

  run { 
    environment {
      variable "DATABASE_URL" { 
        value = "psql://${services.api.vars.db_user}:${services.api.vars.db_pass}@${services.api.containers.db.ip}/${services.api.vars.db_name}"
      }
    }
    commands = [
      "ls",
      "pwd",
      "pip install -r requirements.txt",
      "flask db upgrade",
      "honcho start -f Procfile.dev"
    ]
  }
}

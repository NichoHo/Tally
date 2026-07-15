# Terraform example: the Postgres instance the ledger would use in AWS.
# Included to show infrastructure-as-code awareness; it is not applied anywhere.
# Local development uses the Postgres container in docker-compose.

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  type    = string
  default = "ap-northeast-1"
}

variable "db_password" {
  type      = string
  sensitive = true
}

resource "aws_db_instance" "ledger" {
  identifier     = "tally-ledger"
  engine         = "postgres"
  engine_version = "16.3"
  instance_class = "db.t4g.micro"

  db_name  = "tally"
  username = "tally"
  password = var.db_password

  allocated_storage     = 20
  max_allocated_storage = 50
  storage_encrypted     = true

  # The ledger is the system of record: point-in-time recovery matters.
  backup_retention_period = 7
  deletion_protection     = true
  skip_final_snapshot     = false
  final_snapshot_identifier = "tally-ledger-final"

  publicly_accessible = false
}

output "ledger_db_endpoint" {
  value = aws_db_instance.ledger.endpoint
}

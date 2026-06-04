schema "zord" {
  charset = "utf8mb4"
  collate = "utf8mb4_0900_ai_ci"
}


table "societies" {
  schema = schema.zord
  column "id_society" {
    null = false
    type = char(26)
  }
  column "name" {
    null = false
    type = char(255)
  }
  column "date_deleted_at" {
    null = true
    type = datetime
  }
  primary_key {
    columns = [column.id_society]
  }
}

# scaffold:generated dummys
table "dummys" {
  schema = schema.zord
  column "id" {
    type = char(26)
    null = false
  }
  column "name" {
    type = char(255)
    null = false
  }
  column "email" {
    type = char(255)
    null = false
  }
  primary_key {
    columns = [column.id]
  }
}
# scaffold:end dummys

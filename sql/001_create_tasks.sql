
CREATE TABLE IF NOT EXISTS tasks (
    id           CHAR(32)       NOT NULL,
    task_type    VARCHAR(128)   NOT NULL,
    status       VARCHAR(16)    NOT NULL,
    payload      JSON           NOT NULL,
    priority     INT            NOT NULL DEFAULT 0,
    order_time   DATETIME(6)    NOT NULL,
    retry_count  INT            NOT NULL DEFAULT 0,
    max_retries  INT            NOT NULL DEFAULT 0,
    created_at   DATETIME(6)    NOT NULL,
    updated_at   DATETIME(6)    NOT NULL,

    PRIMARY KEY (id),

    KEY idx_claim (
        task_type,
        status,
        order_time,
        priority DESC,
        id
    )
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci;

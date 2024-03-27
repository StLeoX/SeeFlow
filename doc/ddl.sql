# Apache Doris

CREATE TABLE IF NOT EXISTS `t_L34`
(
    time                DATETIME(6),
    namespace           VARCHAR(127),
    src_identity        BIGINT,
    dest_identity       BIGINT,
    is_reply            BOOLEAN,
    traffic_direction   VARCHAR(15),
    traffic_observation VARCHAR(15),
    verdict             VARCHAR(15)
) DISTRIBUTED BY HASH(src_identity, dest_identity) BUCKETS 32
    PROPERTIES ("replication_num" = "1");


CREATE TABLE IF NOT EXISTS `t_L7`
(
    id            CHAR(36),
    trace_id      CHAR(16),
    src_identity  BIGINT,
    dest_identity BIGINT,
    start_time    DATETIME(6),
    end_time      DATETIME(6)
) DISTRIBUTED BY HASH(src_identity, dest_identity) BUCKETS 32
    PROPERTIES ("replication_num" = "1");


CREATE TABLE IF NOT EXISTS `t_Ep`
(
    namespace VARCHAR(127),
    pod_name  VARCHAR(127),
    svc_name  VARCHAR(127),
    endpoint  BIGINT,
    identity  VARCHAR(127),
    state     VARCHAR(15),
    ip        VARCHAR(15)
) DISTRIBUTED BY HASH(endpoint) BUCKETS 32
    PROPERTIES ("replication_num" = "1");

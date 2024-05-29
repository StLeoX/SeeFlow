-- GET 相关流量
select *
from `t_L34`
where namespace = 'deepflow-spring-demo';

select *
from `t_L7`;

-- DROP t_*
drop table `t_Ep`;
drop table `t_L34`;
drop table `t_L7`;
drop table `t_Sock`;

truncate table `t_L34`;
truncate table `t_L7`;
truncate table `t_Sock`;

-- 验证 cgroup_id 的相关性
select distinct cgroup_id, src_identity, dest_identity
from `t_Sock`
where namespace = 'deepflow-spring-demo';

select *
from t_Sock
where namespace = '';

-- JOIN t_L34 and t_Sock
select t_L34.time,
       t_L34.src_identity,
       t_L34.dest_identity,
       t_L34.is_reply,
       t_L34.traffic_direction,
       t_L34.traffic_observation,
       t_L34.verdict
from t_L34
         join t_Sock
              on t_L34.namespace = t_Sock.namespace
                  and t_L34.src_identity = t_Sock.src_identity
                  and t_L34.dest_identity = t_Sock.dest_identity
where t_L34.namespace = 'deepflow-spring-demo'
  and t_L34.time < '2024-04-18 07:40:00.000000'
  and t_Sock.time < '2024-04-18 07:40:00.000000'
order by t_L34.time
;


with span1 as (select *
               from t_L7
               where id <> ''
               limit 1)
select t_Sock.time
from t_Sock,
     span1
where t_Sock.src_identity = span1.src_identity
  and t_Sock.dest_identity = span1.dest_identity
  and time > span1.start_time
  and time < span1.end_time
;

-- match ids
with span1 as (select *
               from t_L7
               where id <> ''
               limit 1)
select t_L34.src_identity, t_L34.dest_identity
from t_L34,
     span1
where t_L34.time > span1.start_time
  and t_L34.time < span1.end_time
;
# where t_L34.src_identity = span1.src_identity
#   and t_L34.dest_identity = span1.dest_identity

-- match times
with l7_1 as (select *
              from t_L7
              where id <> ''
              limit 1) -- 实际是第二条
select t_L34.*
from t_L34,
     l7_1
where t_L34.src_identity = l7_1.src_identity
  and t_L34.dest_identity = l7_1.dest_identity
  and t_L34.time < l7_1.start_time
;

with l7_1 as (select *
              from t_L7
              where id <> ''
              limit 1)
select t_L34.src_identity,
       t_L34.dest_identity,
       l7_1.src_identity,
       l7_1.dest_identity
from t_L34,
     l7_1
where namespace = 'deepflow-spring-demo'
  and t_L34.time > l7_1.start_time
  and t_L34.time < l7_1.end_time
;

-- ---------- t_Sock

-- match times
with l7_1 as (select *
              from t_L7
              where id <> ''
              limit 1) -- 实际是第二条
select t_Sock.*
from t_Sock,
     l7_1
where t_Sock.src_identity = l7_1.src_identity
  and t_Sock.dest_identity = l7_1.dest_identity
;
-- 结论：t_Sock.time 都小于 l7_1.start_time

with l7_1 as (select *
              from t_L7
              where id <> ''
              limit 1) -- 实际是第二条
select t_Sock.*
from t_Sock,
     l7_1
where t_Sock.src_identity = l7_1.dest_identity
  and t_Sock.dest_identity = l7_1.src_identity
;

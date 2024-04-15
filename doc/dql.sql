-- GET 相关流量

select *
from `t_L34`
where namespace = 'deepflow-spring-demo';

select *
from `t_L7`;

-- DROP
drop table `t_Ep`;
drop table `t_L34`;
drop table `t_L7`;
drop table `t_Sock`;

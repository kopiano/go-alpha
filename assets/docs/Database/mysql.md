# mysql

```shell
$ mysql -u root -p
```


```sql
-- 创建数据库
CREATE DATABASE IF NOT EXISTS `test` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

-- 创建表
CREATE TABLE IF NOT EXISTS `test`.`user` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(45) NOT NULL,
  `age` INT NOT NULL,
  `create_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`))
ENGINE = InnoDB
DEFAULT CHARACTER SET = utf8mb4
COLLATE = utf8mb4_general_ci;

-- 插入数据
INSERT INTO `test`.`user` (`name`, `age`) VALUES ('张三', 18);
INSERT INTO `test`.`user` (`name`, `age`) VALUES ('李四', 20);
INSERT INTO `test`.`user` (`name`, `age`) VALUES ('王五', 22);
```
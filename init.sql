-- create admin user after starting the application at first time.
-- username: admin
-- password: admin123
INSERT INTO
  `mu_user` (
    `email`,
    `username`,
    `password`,
    `status`,
    `roles`,
    `ticket`,
    `created`,
    `updated`
  )
VALUES
  (
    'admin@example.com',
    'admin',
    '$2a$10$fDCCdMXW.29afdMLvXnHt.xBiEHWFMEgmGFa8rI1qrwhzSp78xMBu',
    1,
    'admin',
    '000000',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
  );

-- insert default system options
INSERT INTO
  `zp_option` (`name`, `opts`, `created`, `updated`)
VALUES
  (
    'core.site',
    '{"name":"ZPan","locale":"en","intro":"ZPan is a simple and efficient private cloud storage system.","invite_required":false}',
    datetime ('now'),
    datetime ('now')
  );
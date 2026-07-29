INSERT INTO stations (id, code, name, line) VALUES
  (1, 'MRI', 'Manggarai', 'Bogor Line'),
  (2, 'TBT', 'Tebet', 'Bogor Line'),
  (3, 'CWG', 'Cawang', 'Bogor Line'),
  (4, 'DPK', 'Depok', 'Bogor Line'),
  (5, 'BGR', 'Bogor', 'Bogor Line'),
  (6, 'SDM', 'Sudirman', 'Bogor Line'),
  (7, 'JUA', 'Juanda', 'Bogor Line'),
  (8, 'THB', 'Tanah Abang', 'Cikarang Line'),
  (9, 'BKS', 'Bekasi', 'Cikarang Line');

INSERT INTO trains (id, train_number, route_name) VALUES
  (1, 'BGR-1001', 'Bogor — Jakarta Kota'),
  (2, 'BGR-1003', 'Bogor — Jakarta Kota'),
  (3, 'BGR-1005', 'Bogor — Jakarta Kota'),
  (4, 'BGR-1007', 'Bogor — Jakarta Kota'),
  (5, 'BKS-2001', 'Bekasi — Kampung Bandan');

INSERT INTO schedules (train_id, station_id, arrival, departure, sequence) VALUES
  (1, 5, '07:32', '07:32', 1), (1, 4, '07:50', '07:51', 2), (1, 3, '08:02', '08:03', 3), (1, 2, '08:07', '08:08', 4), (1, 1, '08:12', '08:13', 5), (1, 6, '08:18', '08:19', 6), (1, 7, '08:24', '08:25', 7),
  (2, 5, '07:47', '07:47', 1), (2, 4, '08:05', '08:06', 2), (2, 3, '08:17', '08:18', 3), (2, 2, '08:22', '08:23', 4), (2, 1, '08:27', '08:28', 5), (2, 6, '08:33', '08:34', 6), (2, 7, '08:39', '08:40', 7),
  (3, 5, '08:02', '08:02', 1), (3, 4, '08:20', '08:21', 2), (3, 3, '08:32', '08:33', 3), (3, 2, '08:37', '08:38', 4), (3, 1, '08:42', '08:43', 5), (3, 6, '08:48', '08:49', 6), (3, 7, '08:54', '08:55', 7),
  (4, 5, '08:17', '08:17', 1), (4, 4, '08:35', '08:36', 2), (4, 3, '08:47', '08:48', 3), (4, 2, '08:52', '08:53', 4), (4, 1, '08:57', '08:58', 5), (4, 6, '09:03', '09:04', 6), (4, 7, '09:09', '09:10', 7),
  (5, 9, '08:00', '08:00', 1), (5, 8, '08:25', '08:26', 2), (5, 1, '08:39', '08:40', 3);

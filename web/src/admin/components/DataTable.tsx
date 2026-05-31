import type { ReactNode } from "react";

import styles from "./DataTable.module.css";

export type Column<Row> = {
  key: string;
  header: ReactNode;
  cell: (row: Row) => ReactNode;
  align?: "left" | "right" | "center";
  tabular?: boolean;
  width?: string;
};

export type DataTableProps<Row> = {
  columns: Column<Row>[];
  rows: Row[];
  rowKey: (row: Row) => string;
  empty?: ReactNode;
  caption?: ReactNode;
};

export function DataTable<Row>({
  columns,
  rows,
  rowKey,
  empty,
  caption,
}: DataTableProps<Row>) {
  if (rows.length === 0 && empty) {
    return <div className={styles.empty}>{empty}</div>;
  }
  return (
    <div className={styles.scroller}>
      <table className={styles.table}>
        {caption && <caption className={styles.caption}>{caption}</caption>}
        <thead>
          <tr>
            {columns.map((c) => (
              <th
                key={c.key}
                scope="col"
                style={{ width: c.width, textAlign: c.align ?? "left" }}
                className={c.tabular ? styles.tabular : undefined}
              >
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={rowKey(row)}>
              {columns.map((c) => (
                <td
                  key={c.key}
                  style={{ textAlign: c.align ?? "left" }}
                  className={c.tabular ? styles.tabular : undefined}
                >
                  {c.cell(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

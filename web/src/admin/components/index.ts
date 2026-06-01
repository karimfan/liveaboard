// Admin component library barrel.
// Pages should import primitives from here, not reach into
// individual files. Internal modules (DataTable types, etc.)
// re-export their types too so consumers don't import deep.

export { Page } from "./Page";
export type { PageProps } from "./Page";

export { PageHeader } from "./PageHeader";
export type { PageHeaderProps } from "./PageHeader";

export { Card } from "./Card";
export type { CardProps } from "./Card";

export { Section } from "./Section";
export type { SectionProps } from "./Section";

export { Stat } from "./Stat";
export type { StatProps } from "./Stat";

export { Chip } from "./Chip";
export type { ChipProps, ChipVariant } from "./Chip";

export { Empty } from "./Empty";
export type { EmptyProps } from "./Empty";

export { Button } from "./Button";
export type { ButtonProps, ButtonVariant } from "./Button";

export { Field } from "./Field";
export type { FieldProps } from "./Field";

export { FormSection } from "./FormSection";
export type { FormSectionProps } from "./FormSection";

export { Tabs } from "./Tabs";
export type { TabItem, TabsProps } from "./Tabs";

export { ActionBar } from "./ActionBar";
export type { ActionBarProps } from "./ActionBar";

export { DataTable } from "./DataTable";
export type { Column, DataTableProps } from "./DataTable";

export { SpacesNav } from "./SpacesNav";
export type { SpacesNavProps } from "./SpacesNav";

export { CommandBar, useGlobalCommandShortcut } from "./CommandBar";
export type { CommandBarProps } from "./CommandBar";

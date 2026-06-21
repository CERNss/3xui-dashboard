declare module 'js-yaml' {
  export interface DumpOptions {
    flowLevel?: number
    indent?: number
    lineWidth?: number
    noRefs?: boolean
    sortKeys?: boolean
  }

  export function dump(input: unknown, options?: DumpOptions): string
  export function load(input: string): unknown
}

import { Remote } from 'comlink'
import { Observable } from 'rxjs'
import { catchError, map, startWith } from 'rxjs/operators'
import * as uuid from 'uuid'

import { transformSearchQuery } from '@sourcegraph/shared/src/api/client/search'
import { FlatExtensionHostAPI } from '@sourcegraph/shared/src/api/contract'
import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { IHighlightLineRange } from '@sourcegraph/shared/src/graphql/schema'
import {
    aggregateStreamingSearch,
    AggregateStreamingSearchResults,
    emptyAggregateResults,
} from '@sourcegraph/shared/src/search/stream'
import { asError } from '@sourcegraph/shared/src/util/errors'
import { renderMarkdown } from '@sourcegraph/shared/src/util/markdown'

import { fetchHighlightedFileLineRanges } from '../../repo/backend'
import { LATEST_VERSION } from '../results/StreamingSearchResults'

export type BlockType = 'md' | 'query' | 'file'

interface BaseBlock<I, O> {
    id: string
    type: BlockType
    input: I
    output: O | null
}

export interface QueryBlock extends BaseBlock<string, Observable<AggregateStreamingSearchResults>> {
    type: 'query'
}

export interface MarkdownBlock extends BaseBlock<string, string> {
    type: 'md'
}

export interface FileBlockInput {
    repositoryName: string
    revision: string
    filePath: string
    lineRange: IHighlightLineRange | null
}

export interface FileBlock extends BaseBlock<FileBlockInput, Observable<string[] | Error>> {
    type: 'file'
}

export type Block = QueryBlock | MarkdownBlock | FileBlock

export type BlockInput =
    | Pick<FileBlock, 'type' | 'input'>
    | Pick<MarkdownBlock, 'type' | 'input'>
    | Pick<QueryBlock, 'type' | 'input'>

export type BlockDirection = 'up' | 'down'

export interface BlockProps {
    isReadOnly: boolean
    isSelected: boolean
    isOtherBlockSelected: boolean
    onRunBlock(id: string): void
    onDeleteBlock(id: string): void
    onBlockInputChange(id: string, blockInput: BlockInput): void
    onSelectBlock(id: string | null): void
    onMoveBlockSelection(id: string, direction: BlockDirection): void
    onMoveBlock(id: string, direction: BlockDirection): void
    onDuplicateBlock(id: string): void
}

export interface BlockDependencies {
    extensionHostAPI: Promise<Remote<FlatExtensionHostAPI>>
}

export class Notebook {
    private blocks: Map<string, Block>
    private blockOrder: string[]

    constructor(initializerBlocks: BlockInput[], private dependencies: BlockDependencies) {
        const blocks = initializerBlocks.map(block => ({ ...block, id: uuid.v4(), output: null }))

        this.blocks = new Map(blocks.map(block => [block.id, block]))
        this.blockOrder = blocks.map(block => block.id)

        this.renderMarkdownBlocks()
    }

    private renderMarkdownBlocks(): void {
        const blocks = this.blocks.values()
        for (const block of blocks) {
            if (block.type === 'md') {
                this.runBlockById(block.id)
            }
        }
    }

    public getBlocks(): Block[] {
        return this.blockOrder.map(blockId => {
            const block = this.blocks.get(blockId)
            if (!block) {
                throw new Error(`Block with id:${blockId} does not exist.`)
            }
            return block
        })
    }

    public setBlockInputById(id: string, { type, input }: BlockInput): void {
        const block = this.blocks.get(id)
        if (!block) {
            return
        }
        if (block.type !== type) {
            throw new Error(`Input block type ${type} does not match existing block type ${block.type}.`)
        }
        // We checked that the existing block and the input block have the same type, so the cast below is safe.
        this.blocks.set(block.id, { ...block, input } as Block)
    }

    public runBlockById(id: string): void {
        const block = this.blocks.get(id)
        if (!block) {
            return
        }
        switch (block.type) {
            case 'md':
                this.blocks.set(block.id, { ...block, output: renderMarkdown(block.input) })
                break
            case 'query':
                this.blocks.set(block.id, {
                    ...block,
                    output: aggregateStreamingSearch(
                        transformSearchQuery({
                            // Removes comments
                            query: block.input.replace(/\/\/.*/g, ''),
                            extensionHostAPIPromise: this.dependencies.extensionHostAPI,
                        }),
                        {
                            version: LATEST_VERSION,
                            patternType: SearchPatternType.literal,
                            caseSensitive: false,
                            trace: undefined,
                        }
                    ).pipe(startWith(emptyAggregateResults)),
                })
                break
            case 'file':
                this.blocks.set(block.id, {
                    ...block,
                    output: fetchHighlightedFileLineRanges({
                        repoName: block.input.repositoryName,
                        commitID: block.input.revision || 'HEAD',
                        filePath: block.input.filePath,
                        ranges: block.input.lineRange
                            ? [block.input.lineRange]
                            : [{ startLine: 0, endLine: 2147483647 }], // entire file,
                        disableTimeout: false,
                    }).pipe(
                        map(ranges => ranges[0]),
                        catchError(() => [asError('File not found')])
                    ),
                })
                break
        }
    }

    public deleteBlockById(id: string): void {
        const index = this.blockOrder.indexOf(id)
        if (index === -1) {
            return
        }
        this.blocks.delete(id)
        this.blockOrder.splice(index, 1)
    }

    public insertBlockAtIndex(index: number, blockToInsert: BlockInput): Block {
        const id = uuid.v4()
        const block = { ...blockToInsert, id, output: null }
        // Insert block at the provided index
        this.blockOrder.splice(index, 0, id)
        this.blocks.set(id, block)
        return block
    }

    public getBlockById(id: string): Block | null {
        return this.blocks.get(id) ?? null
    }

    public moveBlockById(id: string, direction: BlockDirection): void {
        const index = this.blockOrder.indexOf(id)
        if ((direction === 'up' && index < 1) || (direction === 'down' && index === this.blockOrder.length - 1)) {
            return
        }
        const swapIndex = direction === 'up' ? index - 1 : index + 1
        const temporaryId = this.blockOrder[swapIndex]
        this.blockOrder[swapIndex] = id
        this.blockOrder[index] = temporaryId
    }

    public duplicateBlockById(id: string): Block | null {
        const block = this.blocks.get(id)
        if (!block) {
            return null
        }
        const index = this.blockOrder.indexOf(id)
        return this.insertBlockAtIndex(index + 1, block)
    }

    public getFirstBlockId(): string | null {
        return this.blockOrder.length > 0 ? this.blockOrder[0] : null
    }

    public getLastBlockId(): string | null {
        return this.blockOrder.length > 0 ? this.blockOrder[this.blockOrder.length - 1] : null
    }

    public getPreviousBlockId(id: string): string | null {
        const index = this.blockOrder.indexOf(id)
        return index >= 1 ? this.blockOrder[index - 1] : null
    }

    public getNextBlockId(id: string): string | null {
        const index = this.blockOrder.indexOf(id)
        return index >= 0 && index < this.blockOrder.length - 1 ? this.blockOrder[index + 1] : null
    }
}

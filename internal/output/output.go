package output

import (
	"encoding/json"
	"fmt"
	"io"
	"iter"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

type Output[T any] struct {
	Format           string
	Paginate         bool
	Items            iter.Seq2[T, error]
	FirstResponse    *gitlab.Response
	Table            *tableprinter.TablePrinter
	TableRowRenderer func(table *tableprinter.TablePrinter, item T)
}

func NewListOutput[T any](format string, items iter.Seq2[T, error], resp *gitlab.Response, paginate bool) *Output[T] {
	return &Output[T]{
		Format:        format,
		Paginate:      paginate,
		Items:         items,
		FirstResponse: resp,
		Table:         tableprinter.NewTablePrinter(),
	}
}

func (o *Output[T]) SetTableRowRenderer(renderer func(table *tableprinter.TablePrinter, item T)) {
	o.TableRowRenderer = renderer
}

func (o *Output[T]) SetTableHeader(str ...any) {
	o.Table.AddRow(str...)
}

func (o *Output[T]) Render(w io.Writer) error {
	switch o.Format {
	case "text":
		return o.renderText(w)
	case "json":
		return o.renderJson(w)
	case "jsonnd":
		return o.renderJsonNd(w)
	default:
		return fmt.Errorf("unknown output format: %s", o.Format)
	}
}

func (o *Output[T]) renderText(w io.Writer) error {
	num := 0
	for item, err := range o.Items {
		if err != nil {
			return err
		}
		o.TableRowRenderer(o.Table, item)
		o.Table.EndRow()
		num++
	}
	table := ""
	if num > 0 {
		table = o.Table.Render()
	}

	// TODO: use NewListTitle
	title := fmt.Sprintf("Showing %d of %d projects (Page %d of %d).\n", num, o.FirstResponse.TotalItems, o.FirstResponse.CurrentPage, o.FirstResponse.TotalPages)

	//TODO: use pager
	//if err := o.IO.StartPager(); err != nil {
	//	return fmt.Errorf("failed to start pager: %q", err)
	//}
	//defer o.IO.StopPager()

	_, err := fmt.Fprintf(w, "%s\n%s\n", title, table)
	return err
}

func (o *Output[T]) renderJson(w io.Writer) error {
	allItems, err := ToSlice(o.Items)
	if err != nil {
		return err
	}

	itemsJson, _ := json.Marshal(allItems)
	_, err = fmt.Fprintln(w, string(itemsJson))
	return err
}

func (o *Output[T]) renderJsonNd(w io.Writer) error {
	for item, err := range o.Items {
		if err != nil {
			return err
		}

		itemJson, _ := json.Marshal(item)
		_, err = fmt.Fprintln(w, string(itemJson))
		if err != nil {
			return err
		}
	}
	return nil
}

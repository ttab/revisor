package revisor

type DefaultValueCollector struct {
	c    *collectorAnnotations
	path []EntityRef
}

type collectorAnnotations struct {
	List []ValueAnnotation
}

func NewValueCollector() *DefaultValueCollector {
	return &DefaultValueCollector{
		c: &collectorAnnotations{},
	}
}

func (c *DefaultValueCollector) CollectValue(a ValueAnnotation) {
	a.Ref = append(c.path, a.Ref...)
	c.c.List = append(c.c.List, a)
}

func (c *DefaultValueCollector) With(ref EntityRef) ValueCollector {
	path := make([]EntityRef, len(c.path), len(c.path)+1)

	_ = copy(path, c.path)

	path = append(path, ref)

	n := DefaultValueCollector{
		c:    c.c,
		path: path,
	}

	return &n
}

func (c *DefaultValueCollector) Values() []ValueAnnotation {
	return c.c.List
}

var _ ValueCollector = &DefaultValueCollector{}

namespace Workspace;

// SharedClass is a class used across multiple files
public class SharedClass : SharedInterface
{
    public int Id;
    public string Name = "";
    public double Value;
    public List<string> Constants = new();

    // Method is a method of SharedClass
    public string Method()
    {
        return Name;
    }

    // Process implements SharedInterface for SharedClass
    public void Process()
    {
        Console.WriteLine($"Processing {Name} with ID {Id}");
    }

    // GetName implements SharedInterface for SharedClass
    public string GetName()
    {
        return Name;
    }
}

// SharedInterface defines behavior implemented across files
public interface SharedInterface
{
    void Process();
    string GetName();
}

public static class SharedConstants
{
    // SharedConstant is used in multiple files
    public const string SharedConstant = "shared value";
}

// SharedType is a custom type used across files
public enum SharedType
{
    First = 1,
    Second = 2,
    Third = 3,
}
